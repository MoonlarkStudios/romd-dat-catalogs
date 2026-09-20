# Known validation issues

## No-Intro pacing tests measured server arrival instead of admission

Status: FIXED

Command: `mise run check` (CI run 35386465405, PR #32).

Root cause: `TestPacingAcrossCatalogs` compared timestamps taken inside the local
HTTP server against a 20 ms client admission gap with 1 ms tolerance. Variable
transport/server scheduling delay can shorten the observed arrival interval even
when admission is correctly paced. The single-catalog pacing test used the same
measurement. The adapter timestamps admission before `client.Do`; server arrival
is not the quantity the adapter promises to space.

Change: test-only RoundTripper instrumentation records the admission timestamp
from the adapter's next deadline and gap. Both tests assert the full configured
gap without tolerance. The server fixture uses its own step counter. Production
adapter behavior and the five-second gap are unchanged.

Validation: both pacing tests passed 30 race-enabled repetitions; full local
`mise run check` passed. A temporary negative control reset the deadline between
catalogs and correctly failed with `cross-catalog request pacing lost`; it was
then removed and the validated test restored.

Agent guidance: measure provider admission at the client boundary. Do not relax
upstream pacing, increase arrival-time tolerances, or accept a retry-only green
run as a fix for this measurement error.


## Datomatic queued exports were reported as changed forms

Status: MITIGATED

Command: `./bin/publisher --output /tmp/nes-check --definitions definitions --catalog no-intro/nes/standard`

Root cause: on 2026-09-20 UTC, a bounded capture of the official anonymous NES
selection response showed an explicit temporary-unavailability/export-queue
notice inside `main_form`. The normal full form still passes strict validation.
The old adapter classified the missing controls as `form_changed` (or a failed
preparation as `prepare_failed`), which did not distinguish provider availability
from actual form-contract drift.

Change: the reviewed queue notice now produces `upstream_pending` at selection,
preparation and manager stages. Synthetic tests cover each stage, no extra
requests, no candidate output, and last-artifact/event retention. Unrelated forms
and partial notices do not match. No retry deadline is invented, no validation is
relaxed, and no automatic retry loop is added.

Agent guidance: inspect the provider response before declaring a parser change.
Do not weaken form checks or repeatedly retry an export that is queued upstream.
Retain the working artifact and verify actual healthy acquisition once generation
finishes. The specific diagnostic is in workflow output; signed public health
continues to use the existing generic acquisition-failure contract.
