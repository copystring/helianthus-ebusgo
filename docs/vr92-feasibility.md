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
