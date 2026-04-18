# Principles

## What cairn is

- A selfhosted uptime monitoring, incident management and status page service.
- Made for homelabs, small teams and small companies.
- Alternative to Statuspage/Instatus/BetterStack but run on your own hardware.
- Open Source (AGPL-3.0)
- Free for everyone.

## What cairn isn't

- A SaaS[^1]
- Paid.
- A metrics system (use Prometheus/Grafan for this)

## Principles/Philosophy

**Self-hosters first**:
Can this run on a crappy VPS or raspberry pi? Is it easy to set up?

**Privacy by default**:
No telementry. No external requests unless the operator explicity configured this. No phone-home.

**Own your data**:
Backup, export, migrate away. No lock-ins

**No dark patterns**:
No upsells, no account required to download. if cairn has a button, it does what it says.

**Customizability**:
Let the user customize this however they want.

## Look and feel

### Aesthetic

- Dark-mode first

Reference: GitHub (dark mode)

Cairn communicates bad news on its best days. The tone is never excited.

### Typography

`Geist` for UI, `Geist Mono` for code/timestamps

### Color

One accent color. No secondary brand color.

Status colors (up/down/degraded) are semantic and consistent across admin and status pages.

Status is never conveyed by color alone. Always paired with icon, text, or position.

#### Surfaces (dark mode)

|                                                           | Token             | Hex       |
| --------------------------------------------------------- | ----------------- | --------- |
| ![#0D1117](https://singlecolorimage.com/get/0D1117/20x20) | `--bg-base`       | `#0D1117` |
| ![#161B22](https://singlecolorimage.com/get/161B22/20x20) | `--bg-surface`    | `#161B22` |
| ![#1F2937](https://singlecolorimage.com/get/1F2937/20x20) | `--bg-elevated`   | `#1F2937` |
| ![#242B36](https://singlecolorimage.com/get/242B36/20x20) | `--bg-hover`      | `#242B36` |
| ![#30363D](https://singlecolorimage.com/get/30363D/20x20) | `--border`        | `#30363D` |
| ![#484F58](https://singlecolorimage.com/get/484F58/20x20) | `--border-strong` | `#484F58` |

#### Text (dark mode)

|                                                           | Token              | Hex       |
| --------------------------------------------------------- | ------------------ | --------- |
| ![#F0F6FC](https://singlecolorimage.com/get/F0F6FC/20x20) | `--text-primary`   | `#F0F6FC` |
| ![#8B949E](https://singlecolorimage.com/get/8B949E/20x20) | `--text-secondary` | `#8B949E` |
| ![#6E7681](https://singlecolorimage.com/get/6E7681/20x20) | `--text-tertiary`  | `#6E7681` |

#### Accent

|                                                           | Token             | Hex       |
| --------------------------------------------------------- | ----------------- | --------- |
| ![#7DD3FC](https://singlecolorimage.com/get/7DD3FC/20x20) | `--accent`        | `#7DD3FC` |
| ![#38BDF8](https://singlecolorimage.com/get/38BDF8/20x20) | `--accent-hover`  | `#38BDF8` |
| ![#0EA5E9](https://singlecolorimage.com/get/0EA5E9/20x20) | `--accent-strong` | `#0EA5E9` |
| ![#0C4A6E](https://singlecolorimage.com/get/0C4A6E/20x20) | `--accent-muted`  | `#0C4A6E` |

#### Status (dark mode)

|                                                           | State       | Foreground |                                                           | Muted background |
| --------------------------------------------------------- | ----------- | ---------- | --------------------------------------------------------- | ---------------- |
| ![#34D399](https://singlecolorimage.com/get/34D399/20x20) | Up          | `#34D399`  | ![#064E3B](https://singlecolorimage.com/get/064E3B/20x20) | `#064E3B`        |
| ![#FBBF24](https://singlecolorimage.com/get/FBBF24/20x20) | Degraded    | `#FBBF24`  | ![#78350F](https://singlecolorimage.com/get/78350F/20x20) | `#78350F`        |
| ![#F87171](https://singlecolorimage.com/get/F87171/20x20) | Down        | `#F87171`  | ![#7F1D1D](https://singlecolorimage.com/get/7F1D1D/20x20) | `#7F1D1D`        |
| ![#A78BFA](https://singlecolorimage.com/get/A78BFA/20x20) | Maintenance | `#A78BFA`  | ![#4C1D95](https://singlecolorimage.com/get/4C1D95/20x20) | `#4C1D95`        |
| ![#9CA3AF](https://singlecolorimage.com/get/9CA3AF/20x20) | Unknown     | `#9CA3AF`  |                                                           | —                |

#### Light mode (status page)

|                                                           | Token              | Hex       |
| --------------------------------------------------------- | ------------------ | --------- |
| ![#FFFFFF](https://singlecolorimage.com/get/FFFFFF/20x20) | `--bg-base`        | `#FFFFFF` |
| ![#F6F8FA](https://singlecolorimage.com/get/F6F8FA/20x20) | `--bg-surface`     | `#F6F8FA` |
| ![#FFFFFF](https://singlecolorimage.com/get/FFFFFF/20x20) | `--bg-elevated`    | `#FFFFFF` |
| ![#0D1117](https://singlecolorimage.com/get/0D1117/20x20) | `--text-primary`   | `#0D1117` |
| ![#57606A](https://singlecolorimage.com/get/57606A/20x20) | `--text-secondary` | `#57606A` |
| ![#D0D7DE](https://singlecolorimage.com/get/D0D7DE/20x20) | `--border`         | `#D0D7DE` |

#### Status (light mode)

|                                                           | State       | Foreground |
| --------------------------------------------------------- | ----------- | ---------- |
| ![#059669](https://singlecolorimage.com/get/059669/20x20) | Up          | `#059669`  |
| ![#D97706](https://singlecolorimage.com/get/D97706/20x20) | Degraded    | `#D97706`  |
| ![#DC2626](https://singlecolorimage.com/get/DC2626/20x20) | Down        | `#DC2626`  |
| ![#7C3AED](https://singlecolorimage.com/get/7C3AED/20x20) | Maintenance | `#7C3AED`  |

### Motion

Minimal. Respect `prefers-reduced-motion`.

Nothing pulses, spins, or demands attention unless something is actually broken.

## Accessibility

- WCAG 2.1 AA is the target for public (status) pages _(it's also the EU legal standard, via BFSG[^2])_
- Best-effort AA for the admin UI
- Full keyboard navigation, visible focus rings, semantic HTML, real <label>s.

## Privacy Defaults

- Self-hosted fonts and assets, no external CDNs
- No analytics, no tracking, no version-check phone-home
- IP truncation available as a config option for logs

## References

Design inspiration: GitHub (dark), Linear, Stripe Dashboard
Functional inspiration: Uptime Kuma, Cachet, Statuspage
Tools: tweakcn (shadcn theme), WebAIM contrast checker, axe DevTools


[^1]: SaaS is short for Software as a Service.

[^2]: The German Accessibility Improvement Act (Barrierefreiheits­stärkungsgesetz)
