"""Regenerate the Windows app icon with Pillow."""

from pathlib import Path

from PIL import Image, ImageDraw

img = Image.new('RGBA', (1024, 1024))
d = ImageDraw.Draw(img)
d.rounded_rectangle((56, 56, 968, 968), radius=225, fill='#171b27')
d.rounded_rectangle((169, 169, 855, 855), radius=185, fill='#d6f477')
d.rounded_rectangle((252, 299, 772, 725), radius=116, outline='#171b27', width=55)
d.line((411, 447, 613, 447), fill='#171b27', width=43)
d.line((512, 376, 512, 518), fill='#171b27', width=43)
for x in (407, 617):
    d.ellipse((x - 25, 591, x + 25, 641), fill='#171b27')
output = Path(__file__).resolve().parents[1] / "app.ico"
img.resize((256, 256), Image.Resampling.LANCZOS).save(
    output, sizes=[(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
)
