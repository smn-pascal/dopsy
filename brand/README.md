# Dopsy brand

Dopsy's identity is quiet, technical, and evidence-led. The mark combines a
lowercase-friendly **D** silhouette, a container boundary, and two diagnostic
lines. It deliberately avoids robots, eyes, sparkles, brains, and Docker's whale.

## Assets

| File | Use |
| --- | --- |
| `dopsy-logo-dark.svg` | Primary lockup on dark forest backgrounds |
| `dopsy-logo-light.svg` | Primary lockup on white or pale backgrounds |
| `dopsy-mark.svg` | App icon, avatar, social icon, and small brand moments |
| `dopsy-logo-mono-*.svg` | One-color print, engraving, or constrained contexts |
| `dopsy-mark-mono-*.svg` | One-color compact mark |
| `favicon.svg` | Browser favicon source |
| `dopsy-social-preview.svg` | Editable 1280 × 640 social-preview source |
| `dopsy-social-preview.png` | Ready-to-upload GitHub and LinkedIn preview |

The files under `apps/web/public/` and `docs/public/` are distribution copies of
the canonical assets in this directory.

## Palette

| Token | Hex | Role |
| --- | --- | --- |
| Forest 950 | `#071A15` | Main application and documentation background |
| Forest 900 | `#0B241C` | Elevated surfaces and the icon field |
| Forest 700 | `#1B4838` | Hairlines and quiet borders |
| Mint 400 | `#7EE2B8` | Brand mark and focused actions |
| Ink | `#F1F7F4` | Primary text on dark backgrounds |
| Mist | `#A9BBB2` | Secondary text |

Mint is a signal, not a fill color for large areas. Error and warning colors are
product semantics and should not be used as brand decoration.

## Spacing and size

- Keep clear space around a logo equal to at least half the icon's height.
- Keep clear space around the standalone mark equal to one quarter of its width.
- Do not render the horizontal logo below 120 px wide.
- Do not render the standalone mark below 20 px. Use `favicon.svg` at favicon sizes.

## Usage

- Use the dark logo on Forest 950/900 and the light logo on white or pale surfaces.
- Prefer the standalone mark where the product name is already visible nearby.
- Preserve the supplied proportions, stroke widths, and colors.
- Never rotate, add glow, add gradients, place the mark inside another container,
  or pair it with AI-themed decorative symbols.
- Write the product name as **Dopsy** in prose and `dopsy` in the logo.
