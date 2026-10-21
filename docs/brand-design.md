# Copiggy icon family

The selected direction is **Little collector**, option 3 from [the icon alternatives](brand-options/README.md): a small peach pig hugging a tilted ivory clipboard. The app icon uses that selected image directly. The companion artwork keeps its simple cocoa outlines, coral details and minimal facial features.

| Surface | Assets | Behavior |
| --- | --- | --- |
| Application | `app-icon.png` | Original selected option 3, with three content strokes |
| Empty history | `empty-state.png`, `empty-state-blink.png` | Three native strokes follow the paper's angle, appear one by one, rest and fade; occasional blink |
| Empty pinned list | `pin-ready.png`, `pin-pressed.png` | The pig pushes a coral pin into a note, pauses and returns to rest |
| Menu bar | `tray-normal.png`, `tray-protected.png` | Simplified Little collector pig holding a tilted clipboard; shield/check when the live Paste Guard is enabled |

Assets live in `resources/brand/`. The color PNGs were generated using the built-in image generation tool with genuine alpha transparency. Packaging builds all ICNS sizes from the selected application image.

The two empty-state illustrations occupy the same 112px native layout. History writing uses short native reveals aligned to the tilted paper, while pinning changes between two aligned poses once every 14 seconds. Timers sleep between gestures; Reduce Motion holds a still frame without scheduled animation. Search with no results retains its magnifying-glass icon. Closing the panel or leaving an empty state resets its animation.

## Menu-bar artwork

The menu-bar mark adapts the selected Little collector into a small monochrome drawing: asymmetric ears, a round snout and a tilted clipboard with short content strokes. Its paths live in `internal/app/tray_icon.go`; the two transparent 72px PNGs are exports of that same code. MyGo presents them at 18pt and AppKit supplies the appropriate template tint.

The protected variant adds a shield/check near the clipboard, separated by transparent space so it remains readable without covering the pig's face. Both variants retain the same canvas and visual position. The badge follows the actual native guard state, including permission loss, rather than the saved preference.

## Generation prompts

The application's original prompt is recorded under **Little collector** in [the alternatives](brand-options/README.md). The following edits produced its active companion assets.

### Empty history — empty-state.png

```text
Use case: precise-object-edit.
Input is the user-selected final Copiggy mascot: a simple peach pig leaning slightly and hugging a tilted ivory clipboard with golden clip.
Make ONLY this change: erase the THREE brown horizontal content strokes inside the clipboard's ivory paper. Fill exactly those stroke areas with the matching blank ivory. Do not remove or modify the clip's round dark dot, pig arms, outline or any other features.
The result is the empty-history base of an animation. Preserve the EXACT original canvas size, transparent silhouette, mascot position, angle, scale, face, open eyes, snout, ears, paws, feet, board outline, clip, colors, line weight, and every other feature. No resizing, cropping, reframing or redrawing. The board must remain tilted at exactly the same angle. Flat simple doodle style unchanged. No new text, content marks, decoration, shading or background. Genuine alpha transparency. It is critical this remains pixel-aligned to the source: only the three content strokes should disappear.
```

### Blink — empty-state-blink.png

```text
Use case: precise-object-edit, aligned animation frame.
This is the approved Copiggy pig hugging a blank tilted clipboard. Change ONLY its two open brown oval eyes to two small relaxed closed eyelid curves, same exact centers, same brown. Preserve all other pixels and the EXACT transparent square canvas, position, scale, tilt, pig face, ears, snout, body, arms, feet, ivory blank paper and golden clip. The clipboard paper must remain blank. No pose change, no new content lines, no lighting, shadows, words, texture, resizing, cropping or reframing. Genuine alpha transparency. The only difference should be the two closed eyelids.
```

### Ready to pin — pin-ready.png

The revised pose places both the note and pressing foreleg at chest level, keeping the snout and cheeks clear. Only the pinning companion assets are replaced; the selected application icon and empty-history artwork stay the same.

```text
Use case: precise-object-edit.
Asset: corrected Copiggy "ready to pin" animation frame.
Input: approved Little collector pig app icon, character/style reference and edit target.
Preserve the exact pig face, ears, eyes, snout, body silhouette, feet, colors, rounded cocoa linework, canvas scale and transparent margin. Make its clipboard-holding pose into a natural chest-level pinning pose:
Lower and slightly shorten the ivory note so its top edge sits BELOW the pig's snout and cheek, around60% of canvas height, and its bottom stays above the feet. Remove the golden clip. Keep the tilted rectangular note and three short cocoa content strokes.
EXACTLY TWO short pig forelegs. The near foreleg comes from the LEFT SIDE OF THE TORSO at chest/belly height, below the cheek, reaching a short distance rightward to the TOP-LEFT margin of the lowered note. Its shoulder/root is around(42%,66%) of the square canvas. Use a very simple rounded mitten/hoof without human fingers or thumb bumps. That hoof rests on a small coral pushpin head around(62%,61%) of canvas. A short needle is visible below the pin, just before entering the paper. The other small hoof holds the note's lower-right edge, as in the reference.
Leave the ENTIRE area around snout, mouth and cheeks untouched and clear: absolutely no arm growing out of the face, no curved line like a moustache under the snout, no hand near eye level, no detached limb, no third arm. The reaching arm should visibly belong to the torso and feel short, relaxed and natural. The next frame will press only this hoof and pin about45pixels downward, keeping its shoulder, body and note fixed.
Same simple friendly doodle. No lettering, new background, decorative marks, props, motion lines, shadows or highlights. Genuine alpha transparency.1254px square matching the input.
```

### Pressed pin — pin-pressed.png

```text
Use case: precise-object-edit.
Asset: second aligned frame of the corrected Copiggy pinning mascot.
Input is the approved chest-level ready pose. Keep this exact image, changing ONLY the near pressing hoof TIP and the coral pushpin. The hoof comes from the torso at chest level on the LEFT of the note, well below the snout. Its two arm-root endpoints MUST remain fixed. Bend just the short wrist so the hoof tip and the pin move downward about45pixels. The pin's needle disappears into the paper and its head rests against the note. The hoof presses lightly onto the pin head, completing the pinning gesture.
Do not shift or redraw the arm root, face, head, body, ears, snout, eyes, feet, note outline, note angle, three writing strokes, or far-right supporting hoof. Do not move any limb upward toward the face or snout. Exactly two forelegs with simple rounded ends, no fingers/thumbs, no extra elbow or arm. No new marks or decoration. Keep identical canvas size1254x1254, same composition, color, linework and genuine alpha background. Only the contact hoof tip and pin move down; everything else stays aligned for animation.
```

### Larger pushpin refinement

The pushpin was enlarged in both poses for clearer recognition at the native 112px size. The approved chest-level foreleg pose remains unchanged. These final edits used the built-in image generation tool and replaced the same two project assets.

#### Ready frame

```text
Use case: precise-object-edit.
Input image is the approved Copiggy "ready to pin" mascot. This is a tiny, local adjustment.
Make ONLY the coral-red pushpin about 50% LARGER, so it is clearly identifiable when the mascot is112px tall. Enlarge the pin primarily to the RIGHT of the hoof and slightly above/below its current head, preserving the exact point where the hoof touches it. Its red cap, short waist, broad lower collar and thin dark needle should be recognizable as a pushpin. Keep the same coral color, cocoa outline and simple rounded doodle look. The needle remains visible just before it enters the note. Do not turn the pin into a different prop or change its orientation.
CRITICAL: the small pig's hoof and arm pose were just approved. DO NOT move, enlarge or redraw the hand/arm, head, face, body, legs, clipboard, paper strokes, other paw, or the overall composition. Keep exact1254x1254canvas, scale, transparency and placement. Change only the small pin and the few pixels immediately surrounding its enlarged outline. No new words, decoration, background, lighting or shading. Genuine alpha transparency.
```

#### Pressed frame

```text
Use case: precise-object-edit.
Two inputs:
Image1 is the TARGET: approved Copiggy PRESSED pin animation frame.
Image2 is a SIZE/COLOR reference ONLY: the matching READY frame whose coral pin has just been enlarged.
Edit ONLY the small coral-red pushpin in image1 to precisely match the larger pin head/cap/collar dimensions, shape and cocoa outline thickness shown in image2. Preserve image1's PRESSED position: pin and hoof sit lower than image2; the needle has disappeared into the paper. The coral pin head should have the SAME height and width in both frames, not change size during animation. Enlarge mainly to the right of the hoof's contact point. Maintain an obvious red pushpin silhouette, no new decoration.
Keep the EXACT original image1 pig hand/foreleg pose, all arm roots, face, body, ears, eyes, snout, feet, supporting paw, note position, angle, outline and3content lines,1254x1254canvas, alpha and scale. Do NOT copy the raised arm or needle from image2. Do NOT move or redraw the pig or note. This is only a pin-size adjustment. No words, background, highlights, shadows or added objects. Genuine alpha transparency.
```
