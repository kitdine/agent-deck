---
status: active
created: 2026-08-27
updated: 2026-08-27
---

# Brand Asset Provenance

This document records the evidence chain for AgentDeck's current robot mark.
It is a provenance record, not a legal opinion, a trademark registration, or a
license grant to third parties.

## Current canonical assets

| Asset | Repository path | SHA-256 |
| --- | --- | --- |
| macOS App Icon source slot, 1024 px | `apps/macos/AgentDeckApp/Assets.xcassets/AppIcon.appiconset/AppIcon-512@2x.png` | `7af145fe92253efa67cfbfb11a4ef88034e3f4a11be15b7060bac9824137b95e` |
| macOS App Icon build resource, 512 px | `apps/macos/AgentDeckApp/Assets.xcassets/AppIcon.appiconset/AppIcon-512.png` | `fb1ad6434b818584d4c1b7bf984c230230cc778face9360c3d319eff9f2a9964` |
| Menu-bar template source, 36 px | `apps/macos/AgentDeckApp/Assets.xcassets/AgentDeckMenuBarIcon.imageset/AgentDeckMenuBarIcon@2x.png` | `0788eb64909ec049c129e12667b75233215bcba6522511e1df15e502c275f76a` |
| Menu-bar template source, 18 px | `apps/macos/AgentDeckApp/Assets.xcassets/AgentDeckMenuBarIcon.imageset/AgentDeckMenuBarIcon.png` | `00f7386fb45da012739d68445adc6bdd5e9ca0b594a1482adbe51b3b982d146b` |

The remaining App Icon slots are deterministic size derivatives. The copies in
`apps/macos/AgentDeckApp/Resources/` mirror the 512 px App Icon and 36 px
menu-bar assets used by the build.

## Generation and selection record

The current robot is not sourced from an identified third-party icon library.
Its recorded chain is:

1. On 2026-08-18, Codex session
   `01a014ca-57e4-7e30-8159-09882a6bc2a9` invoked OpenAI's built-in ImageGen
   tool to create AgentDeck macOS prototype boards from project screenshots and
   text instructions. The prompt prohibited copying CodeBurn branding or logo.
2. The generated prototype sequence produced the orange robot candidate. In the
   same session, the operator supplied a crop of that candidate as Image 3 and
   explicitly selected it for consistent menu-bar and popover identity. On
   2026-08-27, the operator separately confirmed that this image was generated
   by Codex rather than obtained from a third-party asset source.
3. A later ImageGen pass produced the selected v5 discussion board. The final
   board was stored during the session as `desktop-surfaces-v5-discussion.png`.
4. At session ordinal 3518, Codex extracted the selected mark with the recorded
   operation `crop=48:42:84:21,scale=120:105:flags=lanczos`, producing
   `agentdeck-robot.png`.
5. Git commit `f7a24f3d001d261f27cfb90825e23c8781b627bd` first preserved that
   prototype source; commit `5a76ce219c7d3e5edf7f9f7c117ed783d7588192` records it as blob
   `9e8d0f76e25db593482a6c941f34b6efbd91f3b8` at
   `docs/topics/desktop-app/ux/prototype/interactive-v7/public/agentdeck-robot.png`.
6. The app implementation removed only the exterior white matte, retained the
   enclosed white face, and generated the required App Icon sizes. A generative
   background-removal candidate was rejected because it redrew the robot and
   was not used. Commit `ce37a7a818e709b2fbbc4f8bff516d9a312370fa` first preserved the App
   Icon set, and `f37328dc077f7b5ab3b01d9d492ab971ab07a155` delivered it with the
   menu-bar application.
7. The current monochrome menu-bar template is a simplified derivative of the
   same selected robot silhouette. It does not introduce a separate third-party
   artwork source.

OpenAI's official
[Image Generation guide](https://platform.openai.com/docs/guides/image-generation)
documents that images can be created from text prompts or generated as part of
a conversation using the image-generation tool. That documentation supports
the technical generation mechanism recorded above; it does not by itself
establish copyrightability, uniqueness, or trademark clearance.

## Rights and use boundary

- AgentDeck treats the current mark as a first-party AI-generated brand asset
  selected and incorporated by the project operator.
- No recorded step identifies a stock-art, emoji, icon-library, or external
  brand asset as the robot source.
- This record does not guarantee that a generated output is unique or that no
  visually similar mark exists.
- Copyright protection for AI-assisted output can differ by jurisdiction. This
  record preserves human selection and the concrete derivative work, but makes
  no claim beyond rights available under applicable law.
- The word mark `AgentDeck` and the robot figurative mark have not been recorded
  here as registered trademarks or as having completed jurisdiction-specific
  clearance.
- Repository access or code reuse does not, through this document alone, grant
  permission to use the AgentDeck name or robot as a product mark.

## Commercial-release checkpoint

Before a commercial release materially relies on the mark, retain the session
and Git evidence above and perform a jurisdiction-appropriate word and
figurative-mark search for the intended software and hosted-service classes.
Record the search date, jurisdictions, classes, databases, and disposition in
this document or a linked legal review. A search-engine or reverse-image-search
miss is not trademark clearance.

Replace the mark through a clean-room design only if the recorded generation
chain is later contradicted, a materially similar earlier mark is found, or
formal counsel recommends replacement.
