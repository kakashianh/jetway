# 05 — Protocol and Codec Design

## 1. Design principles

Jetway separates universal wire syntax from message grammar. Syntax packages validate framing and character/service rules; message profiles interpret airline business messages. This prevents a carrier-specific dialect from weakening a universal parser and allows profiles to be extended per bilateral link.

The implementation is independent and is not affiliated with or endorsed by IATA, A4A, SITA or ARINC. Some message compositions are inferred because paid normative publications were not purchased. Do not claim full conformance where the code documents an extensible profile.

## 2. Type B envelope — `pkg/typeb`

Type B is a teletype envelope commonly used by store-and-forward airline networks. The parser handles:
- optional `ZCZC` and `NNNN` framing;
- priority and one or more 7-character TTY addresses;
- origin address and DDHHMM time group;
- optional header/body separation;
- ITA2 and IA5 repertoires;
- PDM possible-duplicate indicator;
- line and total-byte limits.

Parsing is intentionally diagnostic/lenient for non-conventional but recognizable input. Encoding validates output constraints and wraps address blocks without splitting an address. Parse normalizes whitespace; use the original raw bytes for audit/replay rather than assuming Parse+Encode is byte-identical.

## 3. AIRIMP — `pkg/airimp`

AIRIMP is the message grammar used over Type B. The implementation is an extensible recognizer profile. It recognizes stable, widely documented elements and preserves unsupported elements as opaque content. Carrier-specific variations belong in profile extensions.

Because the paid AIRIMP publication is the normative authority and is not available to the project, this implementation must be described as a functional profile, not certified/full conformance. The repository identifies the AIRIMP divide message as a notable missing capability in the current profile.

## 4. UN/EDIFACT — `pkg/edifact`

The EDIFACT package implements ISO 9735 syntax concerns including:
- service string advice / UNA characters;
- release and component/element separators;
- repetitions and segment parsing;
- interchange/message envelope validation;
- bounded segment parsing;
- CONTRL syntax/service report construction and consumption.

Syntax is distinct from PADIS segment composition. The parser should reject implausible service characters and enforce configured limits to avoid unbounded resource consumption.

## 5. PADIS — `pkg/padis`

PADIS maps airline reservation and related message structures over EDIFACT. The implementation uses an extensible segment-handler profile. Publicly documented structures such as PNRGOV have stronger public implementation-guide grounding; other message compositions are inferred where paid directories are unavailable.

The profile handles reservation, ticket-control and related segment patterns. Segment mappings should be reviewed against partner ICDs and normative material before production interline use.

## 6. Other protocol families

| Package | Domain / format |
|---|---|
| `avs` | Availability Status messages |
| `ssim` | SSM/ASM schedule messages and SSIM chapter 7 file |
| `ndc` | NDC order operations over HTTP |
| `matip` | MATIP RFC 2351 packet/session handshake and Type B data |
| `mvt` | MVT/MVA/DIV aircraft movement |
| `pnl` | PNL/ADL passenger name lists |
| `dcs` | Departure control, check-in, boarding and load-control messages |
| `aftn` | AFTN envelope |
| `ats` | ICAO ATS FPL/DEP/ARR/DLA/CNL/CHG |
| `acars` | ARINC 620 OOOI reports |
| `paxlst` | EDIFACT passenger-list exchange |
| `iatci` | Inter-airline through-check-in dialogue |
| `baggage` | BSM/BPM/BUM and tracing messages |
| `atfm` | ADEXP air traffic flow-management slot messages |

The full package inventory also includes domain modules such as inventory, fare, crew, BSP, proration and operations.

## 7. Codec contract

A codec should:
1. accept bounded input;
2. return parsed values and structured diagnostics;
3. preserve unknown/unrecognized content where the protocol allows;
4. avoid hidden network/database dependencies;
5. expose encoding validation rather than silently corrupting data;
6. have unit, fuzz and round-trip tests appropriate to the format.

## 8. Character set and normalization

Encoding rules differ by protocol. For example, UNOA does not include lowercase letters. A value accepted by the canonical model may still require protocol-specific validation or sanitization on egress. Do not assume UTF-8 input can be copied directly into every wire format.

## 9. Conformance and provenance

- EDIFACT syntax is based on public UN/ISO 9735 material.
- PNRGOV mapping uses the publicly available IATA implementation guide.
- MATIP follows RFC 2351.
- AIRIMP message composition and several PADIS directories remain profiles due to unavailable paid publications.
- Bilateral carrier dialects may diverge from the base profile; record the partner ICD and profile version used for each production link.

## 10. Protocol evolution

Add a profile extension rather than modifying universal syntax when only one carrier differs. Add shared status codes only in the canonical vocabulary. Add regression fixtures from real/synthetic sanitized traffic and preserve the original sample bytes. A parser change that alters historical interpretation should be accompanied by replay compatibility notes.
