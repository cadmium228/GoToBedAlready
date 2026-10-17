import argparse
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile


def build():
    parser = argparse.ArgumentParser()
    parser.add_argument('--version', default='dev')
    parser.add_argument('--output', default='dist')
    args = parser.parse_args()
    if not all(character.isalnum() or character in '.-_' for character in args.version):
        parser.error('version must contain only letters, digits, dots, dashes or underscores')
    root = Path(__file__).resolve().parents[1]
    output = (root / args.output).resolve()
    output.mkdir(parents=True, exist_ok=True)
    toolchain = subprocess.check_output(['go', 'version'], text=True).strip()
    revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
    hashes = []
    for architecture in ('amd64', 'arm64'):
        with tempfile.TemporaryDirectory() as temporary:
            stage = Path(temporary) / 'GoToBedAlready'
            stage.mkdir()
            environment = dict(os.environ, GOOS='windows', GOARCH=architecture, CGO_ENABLED='0')
            subprocess.run([
                'go', 'build', '-trimpath', '-buildvcs=false',
                '-ldflags', f'-s -w -X main.version={args.version}',
                '-o', str(stage / 'GoToBedAlready.exe'), './cmd/gotobedalready'
            ], cwd=root, env=environment, check=True)
            shutil.copytree(root / 'web', stage / 'web')
            (stage / 'scripts').mkdir()
            shutil.copy(root / 'scripts' / 'observe.ps1', stage / 'scripts')
            shutil.copy(root / 'README.md', stage)
            (stage / 'BUILD.txt').write_text(f'GoToBedAlready {args.version}\n{toolchain}\nCommit: {revision}\nTarget: windows/{architecture}\n', encoding='utf-8')
            archive = output / f'GoToBedAlready-{args.version}-windows-{architecture}.zip'
            with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED) as bundle:
                for file in sorted(stage.rglob('*')):
                    if file.is_file():
                        bundle.write(file, file.relative_to(stage.parent))
            hashes.append(f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}')
            print(archive)
    (output / 'SHA256SUMS.txt').write_text('\n'.join(hashes) + '\n', encoding='utf-8')


if __name__ == '__main__':
    build()
