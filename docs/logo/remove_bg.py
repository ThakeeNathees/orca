import cv2
import numpy as np
import sys

# Usage: python remove-bg.py <none|white>

INPUT_FILENAME = 'logo-nano-banana.png'
VALID_MODES = ['none', 'white', 'white-none']

if len(sys.argv) != 2 or sys.argv[1] not in VALID_MODES:
    print(f"Usage: python {sys.argv[0]} <{'|'.join(VALID_MODES)}>")
    sys.exit(1)

mode = sys.argv[1]
OUTPUT_FILENAME = f'logo-bg-{mode}.png'

img = cv2.imread(INPUT_FILENAME)
if img is None:
    print(f"Error: Could not read '{INPUT_FILENAME}'.")
    sys.exit(1)

print(f"Processing '{INPUT_FILENAME}' -> '{OUTPUT_FILENAME}'...")

# Convert to grayscale and blur slightly to reduce noise
gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
blurred = cv2.GaussianBlur(gray, (3, 3), 0)

# Otsu's thresholding: subject (dark) becomes white(255), background becomes black(0)
_, thresholded_mask = cv2.threshold(blurred, 0, 255, cv2.THRESH_BINARY_INV + cv2.THRESH_OTSU)

# Clean up edges with morphological operations
kernel = np.ones((3, 3), np.uint8)
mask_cleaned = cv2.morphologyEx(thresholded_mask, cv2.MORPH_OPEN, kernel, iterations=1)
mask_final = cv2.morphologyEx(mask_cleaned, cv2.MORPH_CLOSE, kernel, iterations=1)

if mode == 'white':
    result = np.full_like(img, (255, 255, 255))
    result[mask_final > 0] = 0
    cv2.imwrite(OUTPUT_FILENAME, result)
elif mode == 'white-none':
    # White foreground on transparent background
    result_rgba = cv2.cvtColor(img, cv2.COLOR_BGR2BGRA)
    result_rgba[:, :, 0:3] = 255  # set RGB to white
    result_rgba[:, :, 3] = mask_final  # alpha from mask
    cv2.imwrite(OUTPUT_FILENAME, result_rgba)
else:
    result_rgba = cv2.cvtColor(img, cv2.COLOR_BGR2RGBA)
    result_rgba[:, :, 3] = mask_final
    result_rgba[mask_final > 0, 0:3] = 0
    cv2.imwrite(OUTPUT_FILENAME, result_rgba)

print(f"Done: '{OUTPUT_FILENAME}'")
