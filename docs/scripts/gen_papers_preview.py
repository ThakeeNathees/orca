"""Generate a stacked preview image of both research papers.

Paper 1 sits at the back, Paper 2 is offset on top. Both have drop shadows.
Output: docs/public/papers-preview.png
"""

from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter

# Config
OFFSET_X = 300
OFFSET_Y = 250
SHADOW_OFFSET = 50
SHADOW_BLUR = 100
SHADOW_COLOR = (0, 0, 0, 40)
BG_COLOR = (255, 255, 255, 0)  # transparent background
SCALE = 0.5  # scale down the paper images

# Resolve paths relative to this script (docs/scripts/ -> docs/public/)
BASE = Path(__file__).resolve().parent.parent / "public"


def add_shadow(img: Image.Image) -> Image.Image:
    """Add a soft drop shadow that gradually fades out."""
    w, h = img.size
    pad = SHADOW_BLUR * 3 + SHADOW_OFFSET
    canvas = Image.new("RGBA", (w + pad, h + pad), (0, 0, 0, 0))

    # Build the shadow by stacking multiple blurred layers with decreasing
    # opacity, so the shadow fades gradually instead of cutting off abruptly.
    shadow_layer = Image.new("RGBA", (w + pad, h + pad), (0, 0, 0, 0))
    draw = ImageDraw.Draw(shadow_layer)
    sx = SHADOW_BLUR + SHADOW_OFFSET
    sy = SHADOW_BLUR + SHADOW_OFFSET
    draw.rectangle([sx, sy, sx + w, sy + h], fill=SHADOW_COLOR)

    # Apply multiple blur passes for a smoother gradient falloff
    for _ in range(3):
        shadow_layer = shadow_layer.filter(ImageFilter.GaussianBlur(SHADOW_BLUR // 2))

    canvas = Image.alpha_composite(canvas, shadow_layer)
    canvas.paste(img, (SHADOW_BLUR, SHADOW_BLUR), img)
    return canvas


def main():
    paper1 = Image.open(BASE / "paper-1.png").convert("RGBA")
    paper2 = Image.open(BASE / "paper-2.png").convert("RGBA")

    # Scale down
    paper1 = paper1.resize((int(paper1.width * SCALE), int(paper1.height * SCALE)), Image.LANCZOS)
    paper2 = paper2.resize((int(paper2.width * SCALE), int(paper2.height * SCALE)), Image.LANCZOS)

    # Add shadows
    p1 = add_shadow(paper1)
    p2 = add_shadow(paper2)

    # Canvas size to fit both with offset
    canvas_w = max(p1.width, p2.width + OFFSET_X)
    canvas_h = max(p1.height, p2.height + OFFSET_Y)
    canvas = Image.new("RGBA", (canvas_w, canvas_h), BG_COLOR)

    # Paper 1 at origin (back)
    canvas.paste(p1, (0, 0), p1)
    # Paper 2 offset on top (front)
    canvas.paste(p2, (OFFSET_X, OFFSET_Y), p2)

    canvas.save(BASE / "papers-preview.png")
    print(f"Saved {BASE}/papers-preview.png ({canvas_w}x{canvas_h})")


if __name__ == "__main__":
    main()
