import hashlib
import io
from pathlib import Path
import urllib.request
import zipfile


URL = 'https://files.pythonhosted.org/packages/3d/25/9491695c22c4842c5b3903b4dc172e0eecf67a27c0af34a71512c9b76a0a/pywebview-6.2.1-py3-none-any.whl'
SHA256 = '9d07275f53894ab4d5e2e0e996227193e7187dec276d9b624dccbce029216b46'


def fetch(destination):
    with urllib.request.urlopen(URL, timeout=60) as response:
        data = response.read()
    if hashlib.sha256(data).hexdigest() != SHA256:
        raise ValueError('WebView2 distribution checksum mismatch')
    files = {
        'webview/lib/Microsoft.Web.WebView2.Core.dll': 'Microsoft.Web.WebView2.Core.dll',
        'webview/lib/Microsoft.Web.WebView2.WinForms.dll': 'Microsoft.Web.WebView2.WinForms.dll',
        'webview/lib/runtimes/win-x64/native/WebView2Loader.dll': 'amd64/WebView2Loader.dll',
        'webview/lib/runtimes/win-arm64/native/WebView2Loader.dll': 'arm64/WebView2Loader.dll',
        'pywebview-6.2.1.dist-info/licenses/LICENSE': 'pywebview-LICENSE.txt',
    }
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        for source, target in files.items():
            path = destination / target
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(archive.read(source))
    print(destination)


if __name__ == '__main__':
    fetch(Path(__file__).resolve().parents[1] / 'runtime' / 'webview2')
