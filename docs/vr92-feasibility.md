# Virtual VR92 feasibility – offline phase

This branch is an **offline-only** preparation for a virtual Vaillant VR 92.

## Confirmed local facts

- System controller: Vaillant VRC 720/3, eBUS identity CTLV3 at target 0x15.
- One heating circuit, system schema 8.
- Zone 1 currently maps to the system controller.
- Candidate VR92 eBUS pair observed elsewhere: initiator 0x30, target 0x35.
- The existing VR92 target implementation supports 07/04 identification, optional B5/09 scan-ID chunks, and caller-supplied deterministic responses.
- It does not implement measured-temperature transfers, a thermostat state machine, or autonomous initiator traffic.

## Evidence gaps before any live test

1. Obtain authenticated physical VR92 traces with a changed room temperature, including request/response direction, acknowledgments and timing.
2. Determine whether the controller accepts a VR92 as the room sensor for zone 1 under system schema 8.
3. Confirm adapter-side responder timing and support for any required second initiator identity.
4. Implement bounded sensor freshness, valid temperature range, diagnostics and safe unavailable behavior.
5. Prove all of the above in offline replay tests before enabling any bus transmission.

## Safety

No eBUS live output, no HA wiring, no zone remapping, and no adaptive-curve setting changes are implemented here. The emulator leaves unknown commands unmatched.

The apparent B5/0A remote temperature command is currently a **hypothesis**, not a validated wire transmission. B524 06/0A/01/0F is a **read of a regulator-held VR92 temperature register**, not proof of the remote-to-regulator transfer.

Source references:
- https://github.com/Project-Helianthus/helianthus-docs-ebus/blob/main/development/target-emulation.md
- https://github.com/Project-Helianthus/helianthus-docs-ebus/blob/main/protocols/vaillant/ebus-vaillant.md
- https://github.com/john30/ebusd-configuration/issues/585

## First offline prototype (2026-11-17)

The branch now includes the following reusable pieces:

- `emulation/vr92_room_sensor.go`: in-memory room sensor with finite-value validation, application-level plausibility bounds (-20 to 50 C), monotonically increasing observation times, explicit invalidation and expiry after a configured interval (no greater than one hour). It has **no** eBUS output.
- `emulation/vr92_register_record.go`: narrow parser for the observed `B524 OP06 GG=0x0A II=1 RR=0x000F` controller-held room-temperature record. The public fixture's bytes `01 0a 0f 00 00 00 5a 41` decode to 13.625 C. This does **not** identify the message that the remote control sends to the main controller.
- Negative and reference-fixture tests for both components, plus an opt-in GitHub Actions offline workflow.

Offline development is intentionally stopped at the boundary of unverified wire messages. A synthesized `B5/0A` response or write would be conjecture, so it is not implemented.

### Validation

- Isolated implementation of the new sensor and parser was checked locally with `go test -race` and `go vet` (pass).
- The **full fork** requires the GitHub Actions checks, which have not yet reported a result. Isolated tests do not establish integration, gateway readiness or physical eBUS acceptance.
- Nothing in this branch is installable as a complete Home Assistant VR92 or safe to activate for heating control.

### Release gate

Do not release an installable, live-bus-enabled artifact until a trace proves the actual VR92-to-controller temperature transfer, the gateway's responder timing is verified for the installed adapter, and the VRC720/3 accepts a remote room sensor for zone 1 under system schema 8. A physical VR92 test unit or an independently authenticated capture would resolve the largest uncertainty.
