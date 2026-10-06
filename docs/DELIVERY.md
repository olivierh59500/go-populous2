# Native main-game delivery audit

This audit describes the register-based translation. It does not establish a
standalone Go engine: the playable runtime still reads the original executable
and retains its memory/register organization. The independent engine migration
is tracked in [GO_ENGINE.md](GO_ENGINE.md).

The main two-disk Populous II conversion runs through `cmd/populous2` in Go and
Ebitengine. The executable's supplied French revision determines original
behavior and resources. `cmd/populous2-native` shares the same launcher;
`cmd/populous2-legacy` retains the inherited diagnostic engine.

Normal play bypasses the original manual statue challenge, which remains
available with `-original-protection`. The PAL interrupt stays at 50 Hz;
gameplay and help-preview work have separate source-calibrated admission
periods. [HOST_PACING.md](HOST_PACING.md) explains this portable timing boundary.

| Requirement | Implementation and authoritative evidence |
|---|---|
| Original resources/presentation | Actual encoded26-resource loader, relocated HUNK/BSS/CODE owners, original320×200 bitmap/menu/font/palette/screens/cursor; full requester/render CPU/DMA references and bounded desktop captures |
| All29 powers and six heroes | Original17500 dispatcher and raw World callbacks; creator/price/debit/marker/cleanup proofs, full FX controller references, all six hero creation/decision/contact/combat paths |
| Followers, inventions and environment | All35 native follower families and30 FX states bound; six neutral selectors90–100, town/scenery/wall/water controllers,512 full AI cases and every source script-catalog command |
| Terrain issue | Actual raise/lower mouse IRQs on four landscapes;80 whole-main frames,52 complete state/pixel checkpoints and468 native nine-corner height readings |
| Campaign and ending | All1,000 records/passwords, exact statistics/score/overflow/divide-exception/award/skip logic; actual381E/B244/B142/B740/reset/controllers; stock natural result and winning32/999→new-world compositions |
| Complete main-frame composition | Stock2,499 original frames→result/UI/reset→10 new-world frames; six campaign worlds across four LAND banks,6,000 complete frames and96 strict checkpoints |
| Original audio |31 samples/133 patterns, actual device/CIA/Paula DMA/PCM and queued/direct/pause/resume callbacks sharing one runtime access owner; independent CPU/register/DMA/PCM references |
| Menus, input and editor | Original startup/chooser/deity/options/serial/file/protection/help/editor bodies;95 portable keys feed source translation, including held Help and keypad-parenthesis aliases; actual desktop runs |
| Saving/loading | Actual GAM DOS/browser/overwrite/error/pointer behavior; synchronous/asynchronous filesystem and20-pass continuation; paired files restore exact native spans with retained live transport and20 unpaused full main frames |
| Two-player behavior | Real TCP desktop endpoints/native serial adapters;6,144 paired main frames, actual road/wall creation, strict map/pool/RNG/clock state, source-local profile fields, actual partial EOF/error/fallback/corrupt-return and explicit retry checks |
| Editor screen export | Original1A55A palette/header/planar rows,108 CPU cases and real32,104-byte ILBM exports |
| Repository delivery | English production comments/technical documents/commits; private references, tools, captures, hosting exchange directories and generated saves excluded locally; validated commits pushed normally |

The final integrated `go test -buildvcs=false ./...` passes (native package
316.756s). `go vet ./...` and the main binary build pass. Meaningful focused
race checks include native audio ownership, full source UI/controllers,
original stock/campaign/terrain comparisons and actual local TCP gameplay,
failures, retry and paired GAM. The final native-default binary exits0 in a
bounded desktop run and its original menu framebuffer was inspected.

Reference coverage is finite and documented in [VALIDATION.md](VALIDATION.md).
It is not a claim to have played every possible combination or all1,000 worlds
to completion. The main-game feature implementation is complete within these
explicit machine boundaries: the supplied French executable layout, portable
OS/raster-clock/TCP services, and host audio rather than analog Amiga circuitry.
The separate Challenge extension is outside the supplied main two-disk game.
