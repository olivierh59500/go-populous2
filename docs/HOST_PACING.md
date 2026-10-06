# Portable host pacing

The PAL interrupt runs at 50 Hz. This does not mean that the original game can
render and simulate a complete game pass 50 times per second. Its 68000 and
blitter perform substantial work between screen swaps. Running every translated
pass once per host interrupt made the game and conquest help previews too fast.

The portable host now admits new gameplay passes every four PAL interrupts
(12.5 passes per second), and help previews every five (10 previews per second).
Input, palette waits, retained modal children and audio continue independently.
Returning from a dialog does not execute a burst of overdue simulation passes.
The host gate never modifies the original simulation counters or saved game.
An already latched help close click bypasses the preview delay. An integration
test boots the original conquest chooser, runs the real help resource/text and
preview bodies for 50 interrupts with ten animation updates, then closes the
requester using a click that remains pending after button release.

The independent CPU measurement corpus contains all 29 admitted help slots,
with 32 redraws each, and six initial gameplay passes on campaign worlds 12, 25,
30, 100, 500 and 999. These worlds are selected by the original password editor
and cover the four landscapes. Registers, resources and source game rules come
from the original executable; no Go execution provides the timing measurements.

Help redraws use approximately 619,000–741,000 CPU clocks, plus a small DMA
budget. At the PAL CPU clock of 7,093,790 Hz their estimated duration is about
88–105 ms. Campaign passes use approximately 287,000–465,000 CPU clocks plus
125,000–131,000 DMA clocks, about 58–84 ms. The four/five interrupt periods are
stable presentation defaults grounded in this work, rather than exact emulator
deadlines. A heavily populated world can have a different native cost.

The DMA budget counts two CPU clocks per enabled 16-bit channel transfer.
Instruction cycle counts come from the local 68000 analysis interpreter.
CPU/DMA overlap, display DMA contention and original bus phase are not modeled;
the combined duration is a calibration estimate, not cycle-exact emulation.
The numeric corpus is in
`internal/populous2/testdata/native_host_cadence_native.json`. Original references,
capture programs and expanded analysis traces remain local.
