#!/usr/bin/env python3
"""
Ed25519 공개키 Multibase 인코딩/디코딩 도구

사용법:
  # Hex → Multibase 인코딩
  python3 multibase_codec.py encode a49648088a5e637a696ff0965763edcab053d5f1abbd191b3c43f8d8b60a4b4d

  # Multibase → Hex 디코딩
  python3 multibase_codec.py decode z6MkqXjpNQy6y5VkxRpF8Fs6f7Auy6YC72isSmFatdK551tY

관련 표준:
  - Multibase:    https://www.w3.org/TR/controller-document/#multibase-0
  - Multicodec:   https://github.com/multiformats/multicodec/blob/master/table.csv
  - Ed25519:      Multicodec 접두어 0xed01
  - Base58btc:    Multibase 접두어 'z'
  - W3C DID:      https://www.w3.org/TR/did-core/#verification-methods
  - OB 3.0:       https://www.imsglobal.org/spec/ob/v3p0/#publickeymultibase
"""

import sys

# Base58btc 알파벳 (Bitcoin 방식)
ALPHABET = b'123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz'

# Ed25519 public key Multicodec 접두어
ED25519_MULTICODEC = bytes([0xed, 0x01])


def b58encode(data: bytes) -> str:
    n = int.from_bytes(data, 'big')
    result = b''
    while n > 0:
        n, r = divmod(n, 58)
        result = ALPHABET[r:r+1] + result
    # leading zeros
    for byte in data:
        if byte == 0:
            result = ALPHABET[0:1] + result
        else:
            break
    return result.decode()


def b58decode(s: str) -> bytes:
    n = 0
    for c in s.encode():
        n = n * 58 + ALPHABET.index(c)
    result = n.to_bytes((n.bit_length() + 7) // 8, 'big')
    # leading zeros
    pad = 0
    for c in s.encode():
        if c == ALPHABET[0]:
            pad += 1
        else:
            break
    return b'\x00' * pad + result


def encode(hex_pubkey: str) -> str:
    """Hex 공개키 → Multibase 인코딩"""
    raw = bytes.fromhex(hex_pubkey)
    if len(raw) != 32:
        raise ValueError(f"Ed25519 공개키는 32바이트여야 합니다 (입력: {len(raw)}바이트)")

    # Multicodec 접두어 + 공개키
    with_codec = ED25519_MULTICODEC + raw

    # Base58btc 인코딩 + Multibase 접두어 'z'
    return 'z' + b58encode(with_codec)


def decode(multibase: str) -> str:
    """Multibase → Hex 공개키 디코딩"""
    if not multibase.startswith('z'):
        raise ValueError(f"지원하지 않는 Multibase 접두어: '{multibase[0]}' (z=base58btc만 지원)")

    # 'z' 제거 후 Base58btc 디코딩
    raw = b58decode(multibase[1:])

    # Multicodec 접두어 확인
    if raw[:2] != ED25519_MULTICODEC:
        raise ValueError(f"Ed25519 Multicodec 접두어(ed01)가 아닙니다: {raw[:2].hex()}")

    pubkey = raw[2:]
    if len(pubkey) != 32:
        raise ValueError(f"공개키가 32바이트가 아닙니다: {len(pubkey)}바이트")

    return pubkey.hex()


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        print("예시:")
        print("  python3 multibase_codec.py encode a49648088a5e637a696ff0965763edcab053d5f1abbd191b3c43f8d8b60a4b4d")
        print("  python3 multibase_codec.py decode z6MkqXjpNQy6y5VkxRpF8Fs6f7Auy6YC72isSmFatdK551tY")
        sys.exit(1)

    command = sys.argv[1]
    value = sys.argv[2]

    if command == 'encode':
        result = encode(value)
        print(f"입력 (Hex):       {value}")
        print(f"출력 (Multibase): {result}")
        print()
        print(f"분해:")
        print(f"  z          = Multibase 접두어 (base58btc)")
        print(f"  ed01       = Multicodec (Ed25519 public key)")
        print(f"  {value}")
        print(f"             = 원본 공개키 (32바이트)")

    elif command == 'decode':
        result = decode(value)
        print(f"입력 (Multibase): {value}")
        print(f"출력 (Hex):       {result}")
        print()
        print(f"분해:")
        print(f"  {value[0]}          = Multibase 접두어 (base58btc)")
        raw = b58decode(value[1:])
        print(f"  {raw[:2].hex()}       = Multicodec (Ed25519 public key)")
        print(f"  {result}")
        print(f"             = 원본 공개키 (32바이트)")

    else:
        print(f"알 수 없는 명령어: {command}")
        print("사용: encode 또는 decode")
        sys.exit(1)


if __name__ == '__main__':
    main()
