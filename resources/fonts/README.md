# Interface fonts

Copiggy bundles Inter for Latin text and Noto Sans SC, TC and JP for regional
Chinese and Japanese glyphs. These files are application resources, not
system-wide font installations. Code and JSON retain the system monospace font.

The eight static TrueType faces are Regular (400) and Medium (500). Static faces
let the native renderer select the actual weight rather than relying on variable
font-axis support. Inter uses the text optical size (`opsz=14`). All glyphs and
Unicode mappings from each upstream regional font are retained; no character
subset is applied. The fonts occupy 47,693,276 bytes (45.5 MiB) in total.

## Source and licenses

Source: [Google Fonts](https://github.com/google/fonts/tree/e7db6ca603fd4862010383ff17ffd6cc4ffac1c0/ofl),
commit `e7db6ca603fd4862010383ff17ffd6cc4ffac1c0`.
The exact source URLs, input/output SHA-256 hashes, sizes, axes and PostScript names
are in `manifest.json`. Each family directory contains the unmodified upstream
`OFL.txt`; distribute that notice with its font files.

## Rebuilding the static faces

Use Python with `fontTools==4.60.2`. Download each source named in the manifest to
a temporary directory and verify its source SHA-256 before instancing. For each
output record, use its `axes` values and destination path:

```python
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont

font = TTFont(source_path, recalcTimestamp=False)
font = instantiateVariableFont(
    font, output_record["axes"], inplace=True, updateFontNames=True
)
font.save(destination_path)
```

Verify each output hash against the manifest. The generated files have no `fvar`
table, carry the requested `OS/2.usWeightClass`, and retain the upstream glyph
count and best Unicode cmap. Do not ship the temporary variable-font inputs as
additional runtime assets.
