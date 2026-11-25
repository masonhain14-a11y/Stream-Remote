"""Regenerate the browser icons with CairoSVG."""

from pathlib import Path

import cairosvg

icons = {
    "app": '<rect x="4" y="5" width="16" height="14" rx="4"/><path d="M8 10h8m-4-2v4"/><path d="M9 15h.01m6 0h.01" stroke-width="2.6"/>',
    "monitor": '<rect x="3" y="4.5" width="18" height="13" rx="2"/><path d="M8 21h8m-4-3.5V21"/>',
    "grid": '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>',
    "mouse": '<rect x="6" y="2.5" width="12" height="19" rx="6"/><path d="M12 6v3"/>',
    "keyboard": '<rect x="2.5" y="5.5" width="19" height="13" rx="2.5"/><path d="M6 10h.01m4 0h.01m4 0h.01m4 0h.01M7 14h.01m3 0h7"/>',
    "remote": '<rect x="7" y="2" width="10" height="20" rx="3"/><circle cx="12" cy="8" r="2.1"/><path d="M10.5 14.5h3m-3 3h3"/>',
    "search": '<circle cx="10.7" cy="10.7" r="6.1"/><path d="m15.2 15.2 4.4 4.4"/>',
    "arrow-up-right": '<path d="M7 17 17 7M8 7h9v9"/>',
    "arrow-right": '<path d="M4 12h15m-6-6 6 6-6 6"/>',
    "cursor": '<path d="m5 3 14 9-7 1.5L9 20 5 3Z"/>',
    "left-click": '<rect x="6" y="2.5" width="12" height="19" rx="6"/><path d="M6 10h12m-6-7v7"/><path stroke-width="2.7" d="M9 7h.01"/>',
    "right-click": '<rect x="6" y="2.5" width="12" height="19" rx="6"/><path d="M6 10h12m-6-7v7"/><path stroke-width="2.7" d="M15 7h.01"/>',
    "move": '<path d="M12 3v18M3 12h18M9 6l3-3 3 3m-6 12 3 3 3-3M6 9l-3 3 3 3m12-6 3 3-3 3"/>',
    "send": '<path d="m21 3-7 18-3.7-7.3L3 10l18-7Zm0 0L10.3 13.7"/>',
    "backspace": '<path d="M9 5h10a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H9l-6-7 6-7Zm3 4 6 6m0-6-6 6"/>',
    "enter": '<path d="M20 5v5a4 4 0 0 1-4 4H4m4-4-4 4 4 4"/>',
    "previous": '<path d="M6 5v14m12-13-9 6 9 6V6Z" fill="currentColor"/>',
    "play": '<path d="M5 5.5 13 12l-8 6.5v-13Z" fill="currentColor"/><path d="M16 6v12m4-12v12" stroke-width="2.3"/>',
    "next": '<path d="M18 5v14M6 6l9 6-9 6V6Z" fill="currentColor"/>',
    "volume-down": '<path d="M3 9h4l5-4.5v15L7 15H3V9Zm13 1a3 3 0 0 1 0 4"/>',
    "mute": '<path d="M3 9h4l5-4.5v15L7 15H3V9Zm13 1 5 5m0-5-5 5"/>',
    "volume-up": '<path d="M3 9h4l5-4.5v15L7 15H3V9Zm13 1a3 3 0 0 1 0 4m2.7-6.4a6 6 0 0 1 0 8.8"/>',
    "up": '<path d="m6 15 6-6 6 6"/>',
    "down": '<path d="m6 9 6 6 6-6"/>',
    "left": '<path d="m15 6-6 6 6 6"/>',
    "right": '<path d="m9 6 6 6-6 6"/>',
    "undo": '<path d="m9 7-5 5 5 5m-5-5h11a4 4 0 0 1 4 4v2"/>',
    "expand": '<path d="M8 4H4v4m12-4h4v4M4 16v4h4m12-4v4h-4"/>',
}

output = Path(__file__).resolve().parents[1] / "web" / "icons"
output.mkdir(exist_ok=True)
for name, paths in icons.items():
    art = (
        '<svg xmlns="http://www.w3.org/2000/svg" width="72" height="72" viewBox="0 0 24 24">'
        '<g stroke="#000" fill="none" stroke-width="1.8" stroke-linejoin="round" stroke-linecap="round">'
        + paths.replace("currentColor", "#000")
        + "</g></svg>"
    )
    cairosvg.svg2png(bytestring=art.encode(), write_to=str(output / f"{name}.png"))
