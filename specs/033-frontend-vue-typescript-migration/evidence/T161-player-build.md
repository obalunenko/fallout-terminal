# T161 — Player production runtime inventory

Date: 2026-09-01 (Asia/Tbilisi)

Result: PASS.

- Runtime path-policy self-test classified every valid and invalid fixture, including absolute/traversal rejection and opaque hashed names containing `overseer`, `wails`, or `bindings`.
- Production build: 218 modules; one nonempty HTML, one hashed JS, one hashed CSS, one hashed Fixedsys font, `.keep`, and 20 exact sound files.
- All 21 governed binary assets are byte-identical to their source files.
- Only `index.html`, the hashed JS, and the hashed CSS were content-scanned; no font or sound binary was treated as text.
- Final policy check: PASS.

Sorted normalized relative inventory:

```text
.keep
assets/Fixedsys-C16VDDoP.ttf
assets/index-DPCC7wYI.js
assets/index-ViXS5qWC.css
index.html
sounds/ambient/obj_computerzax_hum_lp.wav
sounds/charscroll/ui_hacking_charscroll.wav
sounds/enter/ui_hacking_charenter_01.wav
sounds/enter/ui_hacking_charenter_02.wav
sounds/enter/ui_hacking_charenter_03.wav
sounds/hack-bad/ui_hacking_passbad.wav
sounds/hack-good/ui_hacking_passgood.wav
sounds/menu-focus/ui_menu_focus.wav
sounds/multiple/ui_hacking_charmultiple_01.wav
sounds/multiple/ui_hacking_charmultiple_02.wav
sounds/multiple/ui_hacking_charmultiple_03.wav
sounds/multiple/ui_hacking_charmultiple_04.wav
sounds/single/ui_hacking_charsingle_01.wav
sounds/single/ui_hacking_charsingle_02.wav
sounds/single/ui_hacking_charsingle_03.wav
sounds/single/ui_hacking_charsingle_04.wav
sounds/single/ui_hacking_charsingle_05.wav
sounds/single/ui_hacking_charsingle_06.wav
sounds/single/ui_hacking_charsingle_07.wav
sounds/single/ui_hacking_charsingle_08.wav
```
