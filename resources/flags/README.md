# Region flags

Source: [lipis/flag-icons v7.3.2](https://github.com/lipis/flag-icons/tree/v7.3.2/flags/4x3), licensed under the [MIT license](LICENSE).

These 4:3 SVG assets cover all 245 numbering regions supported by `github.com/nyaruka/phonenumbers/v2` v2.0.15. Filenames use lowercase region codes. Unknown (`ZZ`) and non-geographical (`001`) numbers have no flag. Assets are embedded in the application; no flags are fetched at runtime.

The official [v7.3.2 source archive](https://codeload.github.com/lipis/flag-icons/tar.gz/refs/tags/v7.3.2) has SHA-256 `6a5497e1029e66415c3941a5e5ee001944b0a7e46bf365c6d5a91a033acee87e`.

Only these adaptations were made:

- Upstream `sh-ac.svg` and `sh-ta.svg` are named `ac.svg` and `ta.svg` to match the numbering-region codes for Ascension Island and Tristan da Cunha. Their contents are unchanged.
- `us.svg` expands the upstream `marker-mid` star placements into 50 local `use` elements because MyGo does not render SVG markers. The original star path, colors, and all 50 positions are preserved. Other artwork is unchanged.

Render the flags as images in their original colors, not monochrome icons.
