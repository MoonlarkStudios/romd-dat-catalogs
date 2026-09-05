from datetime import datetime, timedelta, timezone
import io
import json
import multiprocessing
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import xml.etree.ElementTree as XML
import zipfile

import publisher as p


NOW = datetime(2026, 9, 5, tzinfo=timezone.utc)
BASE = "https://catalogs.example.invalid/"


def dat(version=1, name="Synthetic Console"):
    return (f'<datafile><header><name>{name}</name><version>{version}</version></header>'
            f'<game name="Game {version}"><rom name="rom.bin" size="1" crc="d202ef8d"/>'
            '</game></datafile>').encode()


def zip_bytes(data, year=2020, name="catalog.dat"):
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w") as archive:
        archive.writestr(zipfile.ZipInfo(name, (year, 1, 1, 0, 0, 0)), data)
    return stream.getvalue()


def concurrent_publish(root, attempts):
    p.publish(root, attempts, BASE, now=NOW)


class PublisherTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.home = Path(self.temp.name)
        self.output = self.home / "output"
        self.input = self.home / "input.dat"
        self.input.write_bytes(dat())
        self.attempt = {"catalogId": "synthetic/console/standard", "expectedName": "Synthetic Console",
                        "sourceUrl": "https://example.invalid/dat", "path": self.input}

    def publish(self, attempts=None, **kwargs):
        return p.publish(self.output, attempts if attempts is not None else [self.attempt],
                         BASE, now=NOW, **kwargs)

    def test_first_subscription_resolves_exact_original_bytes(self):
        snapshot = self.publish()
        loaded = p.load_snapshot(self.output)
        self.assertEqual(snapshot, loaded)
        candidates = p.catch_up(loaded, {})
        self.assertEqual(dat(), p.verified_read(self.output, candidates[self.attempt["catalogId"]]))
        self.assertEqual("unsigned-development", json.loads((self.output / "current.json").read_text())["trust"])

    def test_repacked_zip_has_no_new_event(self):
        self.input.write_bytes(zip_bytes(dat()))
        first = self.publish()
        self.input.write_bytes(zip_bytes(dat(), 2025))
        second = self.publish()
        self.assertEqual(first["events"], second["events"])
        self.assertEqual(first["catalogs"], second["catalogs"])

    def test_missed_versions_catch_up_despite_truncated_feed(self):
        first = self.publish()
        old_hash = first["events"][0]["artifact"]["sha256"]
        for version in (2, 3, 4):
            self.input.write_bytes(dat(version))
            latest = self.publish(feed_limit=1)
        self.assertEqual(1, len(latest["events"]))
        candidates = p.catch_up(latest, {self.attempt["catalogId"]: old_hash})
        self.assertEqual(p.digest(dat(4)), candidates[self.attempt["catalogId"]]["sha256"])
        self.assertEqual({}, p.catch_up(latest, {self.attempt["catalogId"]: p.digest(dat(4))}))

    def test_rejected_version_not_immediately_reapplied(self):
        latest = self.publish()
        self.assertEqual({}, p.catch_up(latest, {}, {self.attempt["catalogId"]: [p.digest(dat())]}))

    def test_restored_old_document_has_new_event(self):
        first = self.publish()
        self.input.write_bytes(dat(2))
        self.publish()
        self.input.write_bytes(dat())
        restored = self.publish()
        self.assertEqual(first["events"][0]["artifact"], restored["events"][0]["artifact"])
        self.assertNotEqual(first["events"][0]["id"], restored["events"][0]["id"])

    def test_partial_failure_retains_working_catalog_and_advances_other(self):
        first = self.publish()
        fail = {k: v for k, v in self.attempt.items() if k != "path"}
        fail["failure"] = "sensitive local diagnostic is not published"
        other = dict(self.attempt, catalogId="synthetic/second/standard")
        second = p.publish(self.output, [fail, other], BASE, now=NOW + timedelta(days=1))
        record = second["catalogs"][self.attempt["catalogId"]]
        self.assertEqual(first["catalogs"][self.attempt["catalogId"]]["artifact"], record["artifact"])
        self.assertEqual(NOW.isoformat(), record["lastSuccessfulCheck"])
        self.assertEqual("acquisition_failed", record["error"])
        self.assertEqual("healthy", second["catalogs"][other["catalogId"]]["health"])

    def test_failure_before_first_success_has_no_artifact(self):
        self.input.unlink()
        snapshot = self.publish()
        record = snapshot["catalogs"][self.attempt["catalogId"]]
        self.assertIsNone(record["artifact"])
        self.assertIsNone(record["lastSuccessfulCheck"])
        self.assertEqual({}, p.catch_up(snapshot, {}))

    def test_missing_from_sweep_does_not_remove_catalog(self):
        first = self.publish()
        second = self.publish([])
        self.assertEqual(first["catalogs"], second["catalogs"])

    def test_variant_mismatch_retains_active_artifact(self):
        first = self.publish()
        self.input.write_bytes(dat(name="Synthetic Console (Decrypted)"))
        second = self.publish()
        record = second["catalogs"][self.attempt["catalogId"]]
        self.assertEqual("identity_mismatch", record["error"])
        self.assertEqual(first["events"], second["events"])

    def test_interruption_before_commit_preserves_previous_current(self):
        first = self.publish()
        old_current = (self.output / "current.json").read_bytes()
        self.input.write_bytes(dat(2))
        def crash():
            raise RuntimeError("injected interruption")
        with self.assertRaises(RuntimeError):
            self.publish(before_commit=crash)
        self.assertEqual(old_current, (self.output / "current.json").read_bytes())
        self.assertEqual(first, p.load_snapshot(self.output))
        self.assertEqual(2, self.publish()["sequence"])

    def test_tampered_artifact_fails_closed(self):
        first = self.publish()
        artifact = first["events"][0]["artifact"]
        (self.output / artifact["path"]).write_bytes(b"tampered")
        with self.assertRaises(ValueError):
            p.load_snapshot(self.output)
        with self.assertRaises(ValueError):
            self.publish()

    def test_rss_enclosure_and_machine_identity_match_artifact(self):
        snapshot = self.publish()
        rss = XML.fromstring(p.verified_read(self.output, snapshot["feed"]))
        item = rss.find("channel/item")
        event = snapshot["events"][0]
        self.assertEqual(event["id"], item.findtext("guid"))
        self.assertEqual(event["artifact"]["sha256"], item.findtext(f"{{{p.NS}}}sha256"))
        self.assertEqual(BASE + event["artifact"]["path"], item.find("enclosure").get("url"))
        self.assertEqual(str(len(dat())), item.find("enclosure").get("length"))

    def test_deterministic_output_independent_of_input_order(self):
        attempts = [self.attempt, dict(self.attempt, catalogId="synthetic/another/standard")]
        first = self.publish(attempts)
        second = p.publish(self.home / "other", list(reversed(attempts)), BASE, now=NOW)
        self.assertEqual(first, second)

    def test_concurrent_publishers_serialize_without_lost_catalogs(self):
        context = multiprocessing.get_context("spawn")
        processes = [context.Process(target=concurrent_publish, args=(self.output,
                     [dict(self.attempt, catalogId=f"synthetic/console{i}/standard")])) for i in range(3)]
        for process in processes:
            process.start()
        for process in processes:
            process.join(15)
            if process.is_alive():
                process.terminate()
                process.join()
                self.fail("concurrent publisher timed out")
            self.assertEqual(0, process.exitcode)
        snapshot = p.load_snapshot(self.output)
        self.assertEqual(3, snapshot["sequence"])
        self.assertEqual(3, len(snapshot["catalogs"]))

    def test_rejects_unsafe_or_ambiguous_archives(self):
        for name in ("../catalog.dat", "/catalog.dat", "nested\\catalog.dat", "catalog.exe"):
            with self.subTest(name=name), self.assertRaises(p.InvalidCandidate):
                p.document(zip_bytes(dat(), name=name), "Synthetic Console")
        stream = io.BytesIO()
        with zipfile.ZipFile(stream, "w") as archive:
            archive.writestr("one.dat", dat())
            archive.writestr("two.dat", dat())
        with self.assertRaises(p.InvalidCandidate):
            p.document(stream.getvalue(), "Synthetic Console")

    def test_rejects_entities_malformed_empty_and_duplicate_games(self):
        inputs = [b"<html>error</html>", b"<broken", b'<datafile><header><name>Synthetic Console</name></header></datafile>',
                  dat().replace(b"</datafile>", b'<game name="Game 1"/></datafile>'),
                  b'<!DOCTYPE datafile [<!ENTITY x SYSTEM "file:///etc/passwd">]>' + dat().replace(b"Game 1", b"&x;")]
        for data in inputs:
            with self.subTest(data=data), self.assertRaises(p.InvalidCandidate):
                p.document(data, "Synthetic Console")

    def test_external_doctype_declaration_does_not_require_network(self):
        raw = Path("fixtures/example.dat").read_bytes()
        data, counts = p.document(raw, "ROMD Synthetic Console")
        self.assertEqual(raw, data)
        self.assertEqual(1, counts["games"])

    def test_size_limits_apply_before_parse_and_decompression(self):
        with patch.object(p, "MAX_INPUT", 2), self.assertRaises(p.InvalidCandidate):
            p.document(dat(), "Synthetic Console")
        with patch.object(p, "MAX_DOCUMENT", 2), self.assertRaises(p.InvalidCandidate):
            p.document(zip_bytes(dat()), "Synthetic Console")

    def test_duplicate_ids_and_registry_rename_rejected(self):
        with self.assertRaises(ValueError):
            self.publish([self.attempt, self.attempt])
        self.publish()
        with self.assertRaises(ValueError):
            self.publish([dict(self.attempt, expectedName="Renamed")])

    def test_clock_regression_rejected(self):
        self.publish()
        with self.assertRaises(ValueError):
            p.publish(self.output, [self.attempt], BASE, now=NOW - timedelta(days=1))

    def test_public_provenance_rejects_credentials_and_query_strings(self):
        for url in ("https://user:password@example.invalid/", "https://example.invalid/?token=secret", "file:///private/data"):
            with self.subTest(url=url), self.assertRaises(ValueError):
                self.publish([dict(self.attempt, sourceUrl=url)])

    def test_corrupt_historical_feed_artifact_prevents_publication(self):
        first = self.publish()
        self.input.write_bytes(dat(2))
        self.publish()
        (self.output / first["events"][0]["artifact"]["path"]).write_bytes(b"corrupt")
        with self.assertRaises(ValueError):
            self.publish()


if __name__ == "__main__":
    unittest.main()
