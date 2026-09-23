from collections import defaultdict
from html.parser import HTMLParser
from pathlib import Path
import json
import subprocess
import tempfile

app = Path(__file__).resolve().parents[1]
cli = app.parents[1] / 'bundlecheck'

class Scripts(HTMLParser):
    def __init__(self):
        super().__init__()
        self.roots = []
    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == 'script' and 'src' in attrs:
            self.roots.append(attrs['src'])

def run(stats, dist, fmt='json'):
    return subprocess.run([str(cli), 'summary', '--stats', str(stats), '--dist', str(dist), '--format', fmt], capture_output=True, text=True)

for name, dist in [('without-maps', app / 'dist/bundlecheck-smoke'), ('with-maps', Path(__file__).parent / 'build-with-maps')]:
    stats, browser = dist / 'stats.json', dist / 'browser'
    metadata = json.loads(stats.read_text())
    outputs = {k: v for k, v in metadata['outputs'].items() if Path(k).suffix in ('.js', '.mjs', '.cjs')}
    scripts = Scripts()
    scripts.feed((browser / 'index.html').read_text())
    initial, pending = set(), list(scripts.roots)
    while pending:
        key = pending.pop()
        if key in initial:
            continue
        initial.add(key)
        pending.extend(e['path'] for e in outputs[key].get('imports', []) if not e.get('external') and e['kind'] == 'import-statement')
    assert len(initial) == 2 and len(outputs) == 3
    expected = {'initialJs': sum(outputs[k]['bytes'] for k in initial), 'lazyJs': sum(v['bytes'] for k, v in outputs.items() if k not in initial), 'totalJs': sum(v['bytes'] for v in outputs.values())}
    result = run(stats, browser)
    assert result.returncode == 0 and not result.stderr, result.stderr
    assert result.stdout == run(stats, browser).stdout
    report = json.loads(result.stdout)
    assert report['schemaVersion'] == '1' and report['toolVersion'] == '0.1.0' and report['command'] == 'summary'
    assert report['summary'] == expected
    packages = defaultdict(lambda: [0, 0])
    for key, output in outputs.items():
        emitted = (browser / key).read_bytes()
        footer = ('//# sourceMappingURL=' + key + '.map\n').encode()
        if name == 'with-maps':
            debug_ids = [line for line in emitted.splitlines(keepends=True) if line.startswith(b'//# debugId=')]
            assert emitted.endswith(footer) and len(debug_ids) == 1
            assert len(emitted) - output['bytes'] == len(debug_ids[0])
        else:
            assert len(emitted) == output['bytes']
        for source, contribution in output['inputs'].items():
            if '/node_modules/' not in '/' + source:
                continue
            parts = source.rsplit('node_modules/', 1)[1].split('/')
            package = '/'.join(parts[:2]) if parts[0].startswith('@') else parts[0]
            packages[package][0 if key in initial else 1] += contribution['bytesInOutput']
    expected_packages = [{'name': k, 'initialBytes': v[0], 'lazyBytes': v[1], 'totalBytes': sum(v)} for k, v in packages.items() if sum(v)]
    expected_packages.sort(key=lambda v: (-v['initialBytes'], v['name']))
    assert report['packages'] == expected_packages
    assert packages['date-fns'][0] == 0 and packages['date-fns'][1] > 0
    text = run(stats, browser, 'text')
    assert text.returncode == 0 and not text.stderr and 'Initial JS' in text.stdout and 'date-fns' not in text.stdout
    (Path(__file__).parent / ('summary-' + name + '.json')).write_text(result.stdout)
    (Path(__file__).parent / ('summary-' + name + '.txt')).write_text(text.stdout)
    print(name, expected, 'packages:', len(report['packages']))
    with tempfile.TemporaryDirectory() as empty:
        failure = run(stats, Path(empty))
        assert failure.returncode != 0 and not failure.stdout and failure.stderr
    failure = run(stats, browser, 'yaml')
    assert failure.returncode != 0 and not failure.stdout and failure.stderr
print('PASS: independent closure, bytes, package attribution, deterministic clean JSON, text, and stderr failure checks')
