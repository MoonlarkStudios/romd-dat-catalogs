"""Local, unsigned DAT publication prototype. No upstream/network acquisition."""

import argparse
import copy
from datetime import datetime, timezone
from email.utils import format_datetime
import fcntl
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import tempfile
from urllib.parse import urlsplit
import xml.etree.ElementTree as XML
import zipfile

from defusedxml import ElementTree as SafeXML
from defusedxml.common import DefusedXmlException

MAX_INPUT = 16 * 1024 * 1024
MAX_DOCUMENT = 32 * 1024 * 1024
MAX_METADATA = 16 * 1024 * 1024
CATALOG_ID = re.compile(r"[a-z0-9]+(?:[./-][a-z0-9]+)*\Z")
OBJECT_PATH = re.compile(r"objects/[0-9a-f]{64}\.(?:dat|json|xml)\Z")
NS = "urn:romd:dat-catalog:experimental:1"
XML.register_namespace("romd", NS)


class InvalidCandidate(ValueError):
    pass


def encode(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False) + "\n").encode()


def digest(data):
    return hashlib.sha256(data).hexdigest()


def bounded_read(path, limit):
    with Path(path).open("rb") as stream:
        data = stream.read(limit + 1)
    if len(data) > limit:
        raise InvalidCandidate("size_limit")
    return data


def document(raw, expected_name):
    """Preserve source bytes; allow only one regular DAT in a bounded ZIP."""
    if len(raw) > MAX_INPUT:
        raise InvalidCandidate("size_limit")
    try:
        if zipfile.is_zipfile(io.BytesIO(raw)):
            with zipfile.ZipFile(io.BytesIO(raw)) as archive:
                members = archive.infolist()
                if len(members) != 1:
                    raise InvalidCandidate("ambiguous_archive")
                member = members[0]
                path = PurePosixPath(member.filename)
                mode = member.external_attr >> 16
                if (member.is_dir() or path.is_absolute() or ".." in path.parts
                        or "\\" in member.filename or path.suffix.lower() != ".dat"
                        or (mode & 0o170000) == 0o120000 or member.flag_bits & 1):
                    raise InvalidCandidate("unsafe_archive")
                if member.file_size > MAX_DOCUMENT:
                    raise InvalidCandidate("size_limit")
                with archive.open(member) as stream:
                    raw = stream.read(MAX_DOCUMENT + 1)
        if len(raw) > MAX_DOCUMENT:
            raise InvalidCandidate("size_limit")
        root = SafeXML.fromstring(raw, forbid_entities=True, forbid_external=True)
        if root.tag != "datafile" or len(root.findall("header")) != 1:
            raise InvalidCandidate("invalid_dat")
        if root.findtext("header/name") != expected_name:
            raise InvalidCandidate("identity_mismatch")
        games = root.findall("game")
        names = [game.get("name") for game in games]
        if not names:
            raise InvalidCandidate("empty_dat")
        if any(not name for name in names) or len(set(names)) != len(names):
            raise InvalidCandidate("invalid_game_identity")
        return raw, {"games": len(games), "roms": len(root.findall("game/rom"))}
    except (DefusedXmlException, XML.ParseError, zipfile.BadZipFile, RuntimeError,
            NotImplementedError, EOFError) as error:
        raise InvalidCandidate("invalid_dat") from error


def atomic_write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=".upload-", dir=path.parent)
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def put_object(root, data, suffix):
    reference = {"path": f"objects/{digest(data)}.{suffix}", "sha256": digest(data), "bytes": len(data)}
    target = root / reference["path"]
    if target.exists():
        if target.read_bytes() != data:
            raise ValueError("immutable object corruption")
    else:
        atomic_write(target, data)
    return reference


def verified_read(root, reference):
    if not OBJECT_PATH.fullmatch(reference["path"]):
        raise ValueError("invalid object path")
    data = bounded_read(root / reference["path"], max(MAX_METADATA, MAX_DOCUMENT))
    if len(data) != reference["bytes"] or digest(data) != reference["sha256"]:
        raise ValueError("object integrity failure")
    return data


def load_snapshot(root):
    """Integrity checks only: this does NOT authenticate a remote publisher."""
    root = Path(root)
    current = json.loads(bounded_read(root / "current.json", MAX_METADATA))
    if current.get("format") != "romd-dat-catalog-experimental-1" or current.get("trust") != "unsigned-development":
        raise ValueError("unsupported metadata")
    snapshot = json.loads(verified_read(root, current["snapshot"]))
    if snapshot["sequence"] != current["sequence"]:
        raise ValueError("sequence mismatch")
    verified_read(root, snapshot["feed"])
    for catalog in snapshot["catalogs"].values():
        if catalog["artifact"]:
            verified_read(root, catalog["artifact"])
    for event in snapshot["events"]:
        verified_read(root, event["artifact"])
    return snapshot


def catch_up(snapshot, installed, rejected=None):
    """Reference fixture consumer: latest eligible version, independent of RSS."""
    rejected = rejected or {}
    return {key: value["artifact"] for key, value in snapshot["catalogs"].items()
            if value["artifact"] and installed.get(key) != value["artifact"]["sha256"]
            and value["artifact"]["sha256"] not in rejected.get(key, [])}


def feed_bytes(events, base_url):
    rss = XML.Element("rss", version="2.0")
    channel = XML.SubElement(rss, "channel")
    for tag, value in [("title", "ROMD DAT catalogs (development)"), ("link", base_url),
                       ("description", "Synthetic, unsigned DAT publication prototype")]:
        XML.SubElement(channel, tag).text = value
    for event in events:
        item = XML.SubElement(channel, "item")
        XML.SubElement(item, "title").text = event["name"]
        XML.SubElement(item, "guid", isPermaLink="false").text = event["id"]
        XML.SubElement(item, "pubDate").text = format_datetime(datetime.fromisoformat(event["publishedAt"]), usegmt=True)
        url = base_url + event["artifact"]["path"]
        XML.SubElement(item, "link").text = url
        XML.SubElement(item, "enclosure", url=url, length=str(event["artifact"]["bytes"]), type="application/xml")
        XML.SubElement(item, f"{{{NS}}}catalogId").text = event["catalogId"]
        XML.SubElement(item, f"{{{NS}}}sha256").text = event["artifact"]["sha256"]
        XML.SubElement(item, f"{{{NS}}}sequence").text = str(event["sequence"])
    return XML.tostring(rss, encoding="utf-8", xml_declaration=True)


def publish(root, attempts, base_url, *, now=None, feed_limit=100, before_commit=None):
    """Accept trusted local registry attempts; commit references only after objects exist."""
    root = Path(root)
    parsed = urlsplit(base_url)
    if parsed.scheme != "https" or not parsed.netloc or parsed.query or parsed.fragment or parsed.username or parsed.password:
        raise ValueError("base URL must be an HTTPS directory without credentials/query/fragment")
    base_url = base_url.rstrip("/") + "/"
    if not 1 <= feed_limit <= 1000:
        raise ValueError("feed_limit must be 1..1000")
    moment = now or datetime.now(timezone.utc)
    if moment.tzinfo is None:
        raise ValueError("timestamp must have a timezone")
    stamp = moment.astimezone(timezone.utc).isoformat()
    ids = [attempt["catalogId"] for attempt in attempts]
    if len(set(ids)) != len(ids) or any(not CATALOG_ID.fullmatch(key) for key in ids):
        raise ValueError("invalid or duplicate catalog ID")
    for attempt in attempts:
        if not attempt.get("expectedName") or not attempt.get("sourceUrl"):
            raise ValueError("registry requires expectedName and sourceUrl")
        source = urlsplit(attempt["sourceUrl"])
        if (source.scheme not in ("http", "https") or not source.netloc
                or source.username or source.password or source.query or source.fragment):
            raise ValueError("sourceUrl must be a public attribution URL without credentials/query/fragment")
        if ("path" in attempt) == ("failure" in attempt):
            raise ValueError("provide a local path or failure, exclusively")
    root.mkdir(parents=True, exist_ok=True)
    # Never unlink the lock: concurrent owners must keep the same inode.
    with (root / ".publish.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        old = load_snapshot(root) if (root / "current.json").exists() else {"sequence": 0, "catalogs": {}, "events": []}
        if old.get("publishedAt", stamp) > stamp:
            raise ValueError("publication clock moved backwards")
        catalogs = copy.deepcopy(old["catalogs"])
        events = []
        sequence = old["sequence"] + 1
        for attempt in sorted(attempts, key=lambda x: x["catalogId"]):
            key = attempt["catalogId"]
            catalog = catalogs.get(key, {"name": attempt["expectedName"], "artifact": None,
                                       "lastSuccessfulCheck": None, "lastChanged": None})
            if catalog["name"] != attempt["expectedName"]:
                raise ValueError("registry identity changed; explicit migration required")
            catalog["lastAttempt"] = stamp
            try:
                if "failure" in attempt:
                    raise InvalidCandidate("acquisition_failed")
                data, counts = document(bounded_read(attempt["path"], MAX_INPUT), attempt["expectedName"])
                artifact = put_object(root, data, "dat")
                if catalog["artifact"] != artifact:
                    event = {"id": f"urn:romd:dat:{sequence}:{key}:{artifact['sha256']}",
                             "catalogId": key, "name": catalog["name"], "sequence": sequence,
                             "publishedAt": stamp, "artifact": artifact}
                    events.append(event)
                    catalog.update(artifact=artifact, lastChanged=stamp, counts=counts,
                                   provenance={"sourceUrl": attempt["sourceUrl"], "acquiredAt": stamp})
                catalog.update(lastSuccessfulCheck=stamp, health="healthy", error=None)
            except (InvalidCandidate, OSError) as error:
                # Do not publish local paths, credential-bearing URLs, or raw provider errors.
                catalog.update(health="failed", error=str(error) if isinstance(error, InvalidCandidate) else "io_failure")
            catalogs[key] = catalog
        events = (events + old["events"])[:feed_limit]
        feed = put_object(root, feed_bytes(events, base_url), "xml")
        snapshot = {"format": "romd-dat-catalog-experimental-1", "sequence": sequence,
                    "publishedAt": stamp, "catalogs": catalogs, "events": events, "feed": feed}
        reference = put_object(root, encode(snapshot), "json")
        # Validate every object referenced by the candidate before committing it.
        verified_read(root, reference)
        verified_read(root, feed)
        for value in catalogs.values():
            if value["artifact"]:
                verified_read(root, value["artifact"])
        for event in events:
            verified_read(root, event["artifact"])
        if before_commit:
            before_commit()
        atomic_write(root / "current.json", encode({"format": snapshot["format"],
                     "trust": "unsigned-development", "sequence": sequence, "snapshot": reference}))
        return snapshot


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--base-url", default="https://catalogs.example.invalid/")
    args = parser.parse_args()
    attempts = json.loads(bounded_read(args.manifest, MAX_METADATA))
    for attempt in attempts:
        if "path" in attempt:
            attempt["path"] = args.manifest.parent / attempt["path"]
    result = publish(args.output, attempts, args.base_url)
    print(json.dumps({"sequence": result["sequence"], "catalogs": len(result["catalogs"]),
                      "failed": sum(c["health"] == "failed" for c in result["catalogs"].values()),
                      "trust": "unsigned-development"}))


if __name__ == "__main__":
    main()
