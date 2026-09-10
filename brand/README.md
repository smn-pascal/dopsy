# Dopsy brand

Dopsy uses a compact container-shaped **D**, two evidence lines, and a custom
lowercase wordmark. The system is designed to stay legible from favicon size to
release artwork.

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
| Forest 950 | `#071A15` | Compact dark brand surfaces and the mark field |
| Forest 900 | `#0B241C` | Dark navigation and high-contrast brand use |
| Forest 700 | `#1B4838` | Borders on dark surfaces |
| Mint 400 | `#7EE2B8` | Mark and focused accents on dark surfaces |
| Paper | `#FFFFFF` | Primary workspace |
| Canvas | `#F5F7F6` | Application background |
| Ink | `#17241F` | Primary text on light surfaces |
| Muted | `#617169` | Secondary text |
| Line | `#DDE5E0` | Quiet borders and dividers |

Green establishes identity; white space carries the interface. Mint is reserved
for focus, status, and compact brand moments—not large page fills. Error and
warning colors are product semantics and should not be used as decoration.

## Spacing and size

- Keep clear space around a logo equal to at least half the icon's height.
- Keep clear space around the standalone mark equal to one quarter of its width.
- Do not render the horizontal logo below 120 px wide.
- Do not render the standalone mark below 20 px. Use `favicon.svg` at favicon sizes.

## Usage

- Use `dopsy-logo-dark.svg` on dark surfaces and `dopsy-logo-light.svg` on
  light surfaces.
- Prefer the standalone mark where the product name is already visible nearby.
- Preserve the supplied proportions, stroke widths, and colors.
- Never rotate, add glow, add gradients, place the mark inside another container,
  or pair it with AI-themed decorative symbols.
- Write the product name as **Dopsy** in prose and `dopsy` in the logo.
