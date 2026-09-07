#!/usr/bin/env python3
"""Download only the pinned upstream release and verify SHA-256 before extraction."""
import hashlib
from pathlib import Path
import sys
import tarfile
import tempfile
import urllib.request

root = Path(__file__).resolve().parents[1]
platform = sys.argv[1]
assets = {
 'android': ('hiddify-lib-android.tar.gz', '6c4841f7aab23eb1fb17831349ecdfc3ca9c31553b8cbe5effd820cb12607f56', root/'android/app/libs'),
 'windows': ('hiddify-lib-windows-amd64.tar.gz','fc610e67d9fdf23da7cc10633f38806d7081a1c120157d1fbe5ac8cc41b315b4',root/'hiddify-core/bin'),
}
name, expected, target = assets[platform]
target.mkdir(parents=True, exist_ok=True)
with tempfile.TemporaryDirectory() as tmp:
 archive = Path(tmp)/name
 urllib.request.urlretrieve('https://github.com/hiddify/hiddify-next-core/releases/download/v4.1.0/'+name,archive)
 if hashlib.sha256(archive.read_bytes()).hexdigest() != expected:
  raise SystemExit('Core checksum mismatch')
 with tarfile.open(archive) as bundle:
  for entry in bundle.getmembers():
   resolved = (target/entry.name).resolve()
   if not resolved.is_relative_to(target.resolve()) or entry.issym() or entry.islnk():
    raise SystemExit('Unsafe archive member')
  bundle.extractall(target, filter='data')
print('Verified Hiddify core 4.1.0 for '+platform)
