#!/usr/bin/env python3
"""Disassemble an extracted CODE payload; addresses are hunk-relative offsets.

This is a linear analysis aid, not a code/data classifier. Populous II embeds
strings and tables in its CODE hunk. Prefer bounded ranges after locating calls.
"""
import argparse
from pathlib import Path

from capstone import Cs, CS_ARCH_M68K, CS_MODE_BIG_ENDIAN, CS_MODE_M68K_000


def number(value):
    return int(value, 0)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", type=Path)
    parser.add_argument("--start", type=number, default=0)
    parser.add_argument("--end", type=number)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    data = args.input.read_bytes()
    end = args.end if args.end is not None else len(data)
    if args.start < 0 or args.start % 2 or end < args.start or end > len(data):
        parser.error("range must be within the payload and start on an even offset")
    decoder = Cs(CS_ARCH_M68K, CS_MODE_BIG_ENDIAN | CS_MODE_M68K_000)
    decoder.skipdata = True
    with args.output.open("x", encoding="utf-8") as output:
        output.write("; Linear M68000 listing; offsets within one hunk.\n")
        output.write("; Data in CODE may be misinterpreted as instructions.\n")
        for instruction in decoder.disasm(data[args.start:end], args.start):
            output.write(
                f"{instruction.address:08x}: {instruction.bytes.hex():24} "
                f"{instruction.mnemonic:10} {instruction.op_str}\n"
            )


if __name__ == "__main__":
    main()
