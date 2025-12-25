"""
Script per creare l'icona arancione per lo stato tracking.
Legge icon.ico, applica una tinta arancione e salva come icon_tracking.ico
"""

from PIL import Image
import struct

def create_orange_icon():
    # Carica l'icona originale
    img = Image.open('icon.ico')

    # Colore arancione target (come nel tema dell'app)
    orange = (255, 107, 43)  # #ff6b2b

    # Crea le due dimensioni necessarie per l'ICO (16x16 e 32x32)
    sizes = [(16, 16), (32, 32)]
    images = []

    for size in sizes:
        # Ridimensiona l'immagine originale
        resized = img.resize(size, Image.Resampling.LANCZOS)

        # Converti in RGBA se necessario
        if resized.mode != 'RGBA':
            resized = resized.convert('RGBA')

        # Applica tinta arancione mantenendo l'alpha
        pixels = resized.load()
        for y in range(size[1]):
            for x in range(size[0]):
                r, g, b, a = pixels[x, y]
                if a > 0:  # Solo pixel visibili
                    # Calcola luminosita' media del pixel originale
                    luminosity = (r + g + b) / 3 / 255
                    # Applica la luminosita' al colore arancione
                    new_r = int(orange[0] * luminosity)
                    new_g = int(orange[1] * luminosity)
                    new_b = int(orange[2] * luminosity)
                    pixels[x, y] = (new_r, new_g, new_b, a)

        images.append(resized)

    # Salva come ICO con entrambe le dimensioni
    images[0].save('icon_tracking.ico', format='ICO', sizes=sizes)
    print("Creata icon_tracking.ico")

    # Genera anche il codice Go per i bytes
    generate_go_bytes('icon_tracking.ico')

def generate_go_bytes(ico_path):
    """Genera il codice Go con i bytes dell'icona"""
    with open(ico_path, 'rb') as f:
        data = f.read()

    # Formatta come array di bytes Go
    lines = []
    for i in range(0, len(data), 16):
        chunk = data[i:i+16]
        hex_bytes = ', '.join(f'0x{b:02x}' for b in chunk)
        lines.append(f'\t{hex_bytes},')

    go_code = 'var iconDataTracking = []byte{\n' + '\n'.join(lines) + '\n}'

    with open('icon_tracking_bytes.txt', 'w') as f:
        f.write(go_code)

    print(f"Generato icon_tracking_bytes.txt ({len(data)} bytes)")

if __name__ == '__main__':
    create_orange_icon()
