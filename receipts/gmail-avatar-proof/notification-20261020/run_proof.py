#!/usr/bin/env python3
"""Replay one frozen sender-photo consumer campaign on the staged disposable stack.

Requires the owned Xvfb :196 and blocked-egress staged server :18085. Never
uses the owner HOME, session bus, tokens, Google grants or a shared installer.
"""
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile
import time
from PIL import Image

OUT = Path(__file__).resolve().parent
ROOT = OUT.parents[2]
BUILD = Path('/tmp/lb-avatar-qt-build')
TOKEN = 'disposable-gphoto-token-20261020'
records = []
work = Path(tempfile.mkdtemp(prefix='lb-photo-notify-'))
(OUT / 'scratch-path.txt').write_text(str(work) + '\n')


def run(name, command, env=None, expected=0):
    started = time.time()
    result = subprocess.run(command, cwd=ROOT, env=env, stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT, text=True, timeout=150)
    (OUT / (name + '.log')).write_text(result.stdout)
    records.append({'name': name, 'argv': command, 'command': shlex.join(command),
                    'exit': result.returncode, 'expected_exit': expected,
                    'start_unix': started, 'end_unix': time.time(),
                    'isolation': {k: env[k] for k in ('HOME', 'DISPLAY', 'XDG_CACHE_HOME',
                                  'DBUS_SYSTEM_BUS_ADDRESS') if env and k in env}})
    (OUT / 'commands.json').write_text(json.dumps(records, indent=2) + '\n')
    print(name, 'exit', result.returncode, flush=True)
    assert result.returncode == expected, name + ': see ' + str(OUT / (name + '.log'))
    return result.stdout


def isolated(name):
    env = {k: v for k, v in os.environ.items()
           if k in ('PATH', 'USER', 'LOGNAME', 'LANG', 'LC_ALL')}
    for variable, directory in [('HOME', 'home'), ('XDG_RUNTIME_DIR', 'runtime'),
                                ('XDG_CONFIG_HOME', 'config'), ('XDG_DATA_HOME', 'data'),
                                ('XDG_CACHE_HOME', 'cache')]:
        path = work / name / directory
        path.mkdir(parents=True, mode=0o700)
        env[variable] = str(path)
    env.update(DISPLAY=':196', DBUS_SYSTEM_BUS_ADDRESS='unix:path=/nonexistent',
               QT_QPA_PLATFORM='xcb', QT_QUICK_BACKEND='software',
               QT_QUICK_CONTROLS_STYLE='org.kde.desktop',
               HTTP_PROXY='http://127.0.0.1:1', HTTPS_PROXY='http://127.0.0.1:1',
               NO_PROXY='127.0.0.1,localhost')
    return env


def painted(path):
    image = Image.open(path).convert('RGB')
    red = (220, 30, 30)
    green = (30, 160, 60)
    hits = sum(image.getpixel((x, y)) == red and
               any(image.getpixel((x + d, y)) == green
                   for d in range(4, min(40, image.width - x)))
               for y in range(image.height) for x in range(image.width))
    assert hits > 100, 'missing actual painted photo: ' + str(path)
    return {'file': str(path.relative_to(OUT)), 'red_with_green_right_pixels': hits,
            'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}


mode = sys.argv[1] if len(sys.argv) > 1 else 'online'
if mode == 'online':
    env = isolated('qml')
    shots = OUT / 'qml-captures'
    shots.mkdir(exist_ok=True)
    env['LB_BUNDLE_PROOF'] = str(shots)
    run('senderphotopaint', ['dbus-run-session', '--', str(BUILD / 'tests/tst_bundleexpand'),
        '-input', str(ROOT / 'qt/tests/tst_senderphotopaint.qml'),
        '-import', str(ROOT / 'qt/tests/qml-imports')], env)
    # The notification break is confined to a disposable QML import copy. The
    # source tree and frozen binary stay untouched; the same pixel oracle must fail.
    import shutil
    control = work / 'negative' / 'qt'
    shutil.copytree(ROOT / 'qt/qml', control / 'qml')
    shutil.copytree(ROOT / 'qt/src/qml', control / 'src/qml')
    shutil.copytree(ROOT / 'qt/tests/qml-imports', control / 'tests/qml-imports')
    source = control / 'qml/InboxView.qml'
    before = source.read_text()
    for signal in ('DataChanged', 'SenderPhotoChanged'):
        needle = 'function on' + signal + '() { Qt.callLater(bundleModel.rebuildPresentation) }'
        assert before.count(needle) == 1
        before = before.replace(needle, 'function on' + signal + '() {}')
    source.write_text(before)
    negative = run('notification-negative', ['dbus-run-session', '--', str(BUILD / 'tests/tst_bundleexpand'),
        '-input', str(ROOT / 'qt/tests/tst_senderphotopaint.qml'),
        '-import', str(control / 'tests/qml-imports')], isolated('negative'), expected=1)
    assert 'first online sender photo must paint without a manual refresh()' in negative
    env = isolated('app')
    profile = work / 'app/profile'
    profile.mkdir()
    (profile / 'profile.json').write_text(json.dumps({'server_url': 'http://127.0.0.1:18085', 'token': TOKEN}))
    (OUT / 'profile-path.txt').write_text(str(profile) + '\n')
    (OUT / 'app-environment.json').write_text(json.dumps(env, indent=2) + '\n')
    run('photo-status', ['curl', '--fail', '--silent', '--show-error', '-H', 'Authorization: Bearer ' + TOKEN,
                        'http://127.0.0.1:18085/v1/gmail/sender-photos/status'])
    run('provider-bytes', ['curl', '--fail', '--silent', '--show-error', '-H', 'Authorization: Bearer ' + TOKEN,
                         'http://127.0.0.1:18085/v1/cards/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/sender-photo',
                         '-o', str(OUT / 'provider-photo.png')])
    assert (OUT / 'provider-photo.png').read_bytes() == (ROOT / 'qt/tests/fixtures/sender-photo-64x64.png').read_bytes()
    captures = OUT / 'app-captures'
    captures.mkdir(exist_ok=True)
    run('online-app', ['dbus-run-session', '--', str(BUILD / 'litterbox-qt'), '--capture-script',
                      str(profile), str(OUT / 'online.json'), str(captures)], env)
    paints = [painted(captures / 'online-first-paint-no-refresh.png'),
              painted(shots / 'online-01-first-paint-no-refresh.png'),
              painted(shots / 'offline-01-cached-paint-fallbacks.png')]
    (OUT / 'paint-online.json').write_text(json.dumps(paints, indent=2) + '\n')
    print('ONLINE, fallback/auth controls, offline-QML and notification-negative passed', flush=True)
elif mode == 'offline':
    previous = json.loads((OUT / 'commands.json').read_text())
    records.extend(previous)
    env = json.loads((OUT / 'app-environment.json').read_text())
    profile = Path((OUT / 'profile-path.txt').read_text().strip())
    captures = OUT / 'offline-app-captures'
    captures.mkdir(exist_ok=True)
    run('offline-app', ['dbus-run-session', '--', str(BUILD / 'litterbox-qt'), '--capture-script',
                       str(profile), str(OUT / 'offline.json'), str(captures)], env)
    (OUT / 'paint-offline.json').write_text(json.dumps(painted(captures / 'offline-cached-no-server.png'), indent=2) + '\n')
    print('OFFLINE cached bytes painted with server stopped', flush=True)
elif mode == 'manifest':
    changed = [ROOT / path for path in ('qt/src/CardStore.cpp', 'qt/qml/InboxView.qml',
                                      'qt/tests/tst_senderphotopaint.qml')]
    artifacts = sorted(path for path in OUT.rglob('*')
                       if path.is_file() and path.name != 'MANIFEST.json')
    rows = [{'path': str(path.relative_to(ROOT)), 'bytes': path.stat().st_size,
             'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
             'kind': 'changed-source' if path in changed else 'new-receipt'}
            for path in changed + artifacts]
    (OUT / 'MANIFEST.json').write_text(json.dumps(rows, indent=2) + '\n')
    print('Durable manifest:', len(changed), 'changed source paths,', len(artifacts), 'receipt files')
else:
    raise SystemExit('mode must be online, offline or manifest')
