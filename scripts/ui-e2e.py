"""Opt-in real GUI test over the Go ui-driver bridge (never Python AF_UNIX).
Requires a freshly built/signed e2e GUI. No core/service/network actions.
"""
from pathlib import Path
import base64
import ctypes
import json
import os
import sqlite3
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[1]
NO_WINDOW = getattr(subprocess, 'CREATE_NO_WINDOW', 0)
VALID = '{"log":{"level":"warn"},"outbounds":[{"type":"direct","tag":"direct"}]}'
EDITED = '{"log":{"level":"error"},"outbounds":[{"type":"direct","tag":"edited-direct"}]}'
INVALID = '{"outbounds":[{"type":"not-a-real-outbound"}]}'


class Session:
    def __init__(self, run, log):
        self.run, self.log = run, log
        self.address = str(run / 'app.sock')
        self.app = self.driver = None
        self.sequence = 0

    def start(self):
        self.app = subprocess.Popen(
            [str(ROOT / 'build/e2e/sing-box.exe'), '--data-dir', str(self.run / 'data')],
            cwd=ROOT, env=dict(os.environ, MYGO_AUTOMATION=self.address),
            stdout=self.log, stderr=self.log, creationflags=NO_WINDOW)
        self.driver = subprocess.Popen(
            [str(ROOT / 'build/ui-driver.exe'), self.address], stdin=subprocess.PIPE,
            stdout=subprocess.PIPE, stderr=self.log, text=True, encoding='utf-8',
            creationflags=NO_WINDOW)
        self.wait(lambda: bool(self.request('windows')['windows']), 'real GUI window')

    def request(self, method, **params):
        self.sequence += 1
        self.driver.stdin.write(json.dumps(dict(id=self.sequence, method=method, params=params)) + '\n')
        self.driver.stdin.flush()
        line = self.driver.stdout.readline()
        if not line:
            raise RuntimeError(f'Go UI driver closed (GUI exit={self.app.poll()}); inspect app.log')
        response = json.loads(line)
        if response.get('id') != self.sequence:
            raise RuntimeError('Unexpected driver response sequence')
        if 'error' in response:
            raise RuntimeError(response['error'])
        return response['result']

    def nodes(self):
        self.request('idle', timeout=2000)
        return self.request('tree')['nodes']

    def matching(self, label, role=None):
        return [n for n in self.nodes() if n.get('label') == label and
                (role is None or n.get('role') == role)]

    def act(self, label, action='press', text=None, role=None):
        def enabled():
            matches = self.matching(label, role)
            return len(matches) == 1 and 'disabled' not in matches[0].get('states', [])
        self.wait(enabled, f'enabled {label}')
        matches = self.matching(label, role)
        if len(matches) != 1:
            raise AssertionError(f'Expected one {label!r} ({role}), found {len(matches)}')
        node = matches[0]
        if 'disabled' in node.get('states', []):
            raise AssertionError(f'Refusing disabled control {label!r}')
        if label in ('启动', '停止', 'Start', 'Stop', '安装服务', '修复服务'):
            raise AssertionError(f'Network/service action outside test scope: {label}')
        params = dict(id=node['id'], action=action)
        if text is not None:
            params['text'] = text
        self.request('act', **params)

    def wait(self, test, description):
        limit = time.monotonic() + 15
        while time.monotonic() < limit:
            if test():
                return
            if self.app.poll() is not None:
                raise AssertionError(f'GUI exited while waiting for {description}')
            time.sleep(.1)
        raise AssertionError(f'Timed out waiting for {description}')

    def shown(self, label, role=None):
        self.wait(lambda: bool(self.matching(label, role)), label)

    def capture(self, name):
        # act() queues a frame; settle and snapshot before capture, otherwise
        # capture can contain the prior page while the later tree is current.
        tree = self.nodes()
        screenshot = self.request('capture')
        (self.run / (name + '.png')).write_bytes(base64.b64decode(screenshot['png']))
        (self.run / (name + '-tree.json')).write_text(
            json.dumps(tree, ensure_ascii=False, indent=2), encoding='utf-8')

    def navigate(self, label):
        if label == '配置':
            self.act('其他', role='menuButton')
            self.shown('管理配置', 'menuItemCheckBox')
            self.act('管理配置', role='menuItemCheckBox')
        else:
            self.act(label, role='button')
        self.nodes()
        if label == '概览':
            self.shown('选择配置', 'menuButton')
            assert not self.matching('HTTP Proxy', 'switch')

    def edit_dialog(self, name, content):
        self.shown('配置名称', 'textField')
        self.act('配置名称', 'setValue', name, role='textField')
        self.act('JSON', 'setValue', content, role='textField')
        self.act('确认', role='button')

    def profile_action(self, name, action):
        self.act('配置操作 ' + name)
        self.shown(action, 'menuItem')
        self.act(action, role='menuItem')

    def close(self, graceful=False):
        if graceful and self.app and self.app.poll() is None:
            # MyGo input injects framework keys, not the Windows system-menu
            # Alt+F4 command. WM_CLOSE reaches the real close handler instead.
            from ctypes import wintypes
            user32 = ctypes.WinDLL('user32', use_last_error=True)
            user32.GetWindowThreadProcessId.argtypes = [wintypes.HWND, ctypes.POINTER(wintypes.DWORD)]
            user32.PostMessageW.argtypes = [wintypes.HWND, wintypes.UINT, wintypes.WPARAM, wintypes.LPARAM]
            callback_type = ctypes.WINFUNCTYPE(wintypes.BOOL, wintypes.HWND, wintypes.LPARAM)
            closed = []
            @callback_type
            def close_owned(hwnd, _):
                pid = wintypes.DWORD()
                user32.GetWindowThreadProcessId(hwnd, ctypes.byref(pid))
                if pid.value == self.app.pid and user32.IsWindowVisible(hwnd):
                    if not user32.PostMessageW(hwnd, 0x0010, 0, 0):
                        return False
                    closed.append(hwnd)
                return True
            user32.EnumWindows.argtypes = [callback_type, wintypes.LPARAM]
            user32.EnumWindows(close_owned, 0)
            if not closed:
                raise AssertionError('No visible window belongs to the test GUI PID')
            # Restart proof requires graceful exit, not a terminate fallback.
            self.app.wait(timeout=8)
        if self.driver:
            if not self.driver.stdin.closed:
                try:
                    self.driver.stdin.close()
                except OSError:
                    pass
            try:
                self.driver.wait(timeout=3)
            except subprocess.TimeoutExpired:
                self.driver.terminate()
                self.driver.wait(timeout=3)
            self.driver = None
        if self.app and self.app.poll() is None:
            self.app.terminate()
            self.app.wait(timeout=5)
        self.app = None


def rows(run):
    # SQLite inspection cannot repair or modify application state.
    with sqlite3.connect((run / 'data/settings.db').as_uri() + '?mode=ro', uri=True) as db:
        return db.execute('SELECT id,name,path FROM profiles ORDER BY id').fetchall()


def selected(run):
    with sqlite3.connect((run / 'data/settings.db').as_uri() + '?mode=ro', uri=True) as db:
        raw = db.execute("SELECT data FROM preferences WHERE name='selected_profile_id'").fetchone()
        return json.loads(raw[0]) if raw else None


def content(run, row):
    file = (run / 'data' / row[2]).resolve()
    assert file.is_relative_to((run / 'data').resolve()), 'Profile path escaped isolated data dir'
    return file


def main():
    for file in ('build/e2e/sing-box.exe', 'build/ui-driver.exe',
                 'build/e2e/resources/daemon/sing-box-daemon.exe'):
        if not (ROOT / file).is_file():
            raise FileNotFoundError(file)
    run = ROOT / 'build' / ('ui-run-' + uuid.uuid4().hex[:8])
    run.mkdir()
    checks = []
    result = dict(passed=False, scope='real-gui-local-config-only', data_dir=str(run / 'data'))
    with (run / 'app.log').open('w', encoding='utf-8') as log:
        ui = Session(run, log)
        try:
            ui.start()
            ui.navigate('配置')
            ui.shown('没有本地配置')
            ui.act('新建配置')
            ui.edit_dialog('UI 实测配置', VALID)
            ui.shown('UI 实测配置', 'button')
            initial = rows(run)
            assert len(initial) == 1 and initial[0][1] == 'UI 实测配置'
            assert json.loads(content(run, initial[0]).read_text(encoding='utf-8')) == json.loads(VALID)
            checks.append('create with official CheckConfig; SQLite and JSON persisted')

            ui.act('导入 JSON')
            ui.edit_dialog('不应保存', INVALID)
            ui.wait(lambda: any('not-a-real-outbound' in n.get('label', '') for n in ui.nodes()),
                    'official semantic validation rejects unknown outbound')
            assert rows(run) == initial
            ui.capture('invalid-outbound')
            ui.act('取消', role='button')
            checks.append('unknown outbound rejected without adding SQLite row')

            ui.act('导入 JSON')
            ui.edit_dialog('导入待删除', VALID)
            ui.shown('导入待删除', 'button')
            imported = next(row for row in rows(run) if row[1] == '导入待删除')
            assert len(rows(run)) == 2 and content(run, imported).is_file()
            checks.append('valid JSON imported through UI and persisted')

            ui.profile_action('UI 实测配置', '编辑内容')
            ui.shown('JSON', 'textField')
            assert json.loads(ui.matching('JSON', 'textField')[0]['value']) == json.loads(VALID)
            ui.act('JSON', 'setValue', EDITED, role='textField')
            ui.act('确认', role='button')
            ui.wait(lambda: json.loads(content(run, initial[0]).read_text(encoding='utf-8')) == json.loads(EDITED),
                    'edited content persisted')
            ui.wait(lambda: not ui.matching('确认', 'button'), 'edit dialog closed')
            ui.shown('UI 实测配置', 'button')
            ui.profile_action('UI 实测配置', '重命名')
            ui.shown('配置名称', 'textField')
            ui.act('配置名称', 'setValue', '重命名后配置', role='textField')
            ui.act('确认', role='button')
            ui.shown('重命名后配置', 'button')
            renamed = next(row for row in rows(run) if row[1] == '重命名后配置')
            assert renamed[0] == initial[0][0] and renamed[2] == initial[0][2]
            checks.append('profile edit and rename preserve SQLite identity/content path')

            ui.profile_action('导入待删除', '删除')
            ui.shown('删除配置？')
            ui.act('取消', role='button')
            assert len(rows(run)) == 2 and content(run, imported).is_file()
            ui.profile_action('导入待删除', '删除')
            ui.shown('删除配置？')
            ui.act('确认', role='button')
            ui.wait(lambda: len(rows(run)) == 1, 'profile deletion')
            assert rows(run) == [renamed] and not content(run, imported).exists()
            ui.act('重命名后配置', role='button')
            ui.wait(lambda: selected(run) == renamed[0], 'selected profile persisted')
            ui.capture('profiles-edited')
            checks.append('delete cancellation preserves data; confirmation removes row/file')

            ui.navigate('分组')
            ui.shown('暂无可用分组')
            ui.capture('groups-empty')
            checks.append('Groups honest empty state without service/core start')
            ui.navigate('设置')
            for label in ('系统 HTTP 代理', '隧道 / 路由', '自动连接'):
                ui.shown(label, 'switch')
                switches = ui.matching(label, 'switch')
                assert len(switches) == 1 and 'disabled' in switches[0].get('states', [])
            ui.capture('settings')
            checks.append('all three network switches exist and remain disabled')

            ui.close(graceful=True)
            ui.start()
            ui.navigate('配置')
            ui.shown('重命名后配置', 'button')
            assert rows(run) == [renamed] and selected(run) == renamed[0]
            assert json.loads(content(run, renamed).read_text(encoding='utf-8')) == json.loads(EDITED)
            assert not ui.matching('导入待删除', 'button')
            ui.capture('profiles-reopened')
            ui.navigate('概览')
            ui.capture('overview-reopened')
            checks.append('edited/renamed/selected profile restored after graceful restart')
            ui.navigate('设置')
            ui.act('跟随系统')
            ui.shown('浅色', 'menuItemCheckBox')
            ui.act('浅色', role='menuItemCheckBox')
            ui.navigate('概览')
            ui.capture('overview-light')
            ui.navigate('设置')
            ui.act('浅色')
            ui.shown('深色', 'menuItemCheckBox')
            ui.act('深色', role='menuItemCheckBox')
            ui.navigate('概览')
            ui.capture('overview-dark')
            checks.append('real Settings appearance menu switches light/dark Overview')
            result['passed'] = True
        except Exception as error:
            result['error'] = str(error)
            try:
                ui.capture('failure')
            except Exception:
                pass
            raise
        finally:
            result['checks'] = checks
            (run / 'result.json').write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf-8')
            print(json.dumps(dict(result, evidence=str(run)), ensure_ascii=False))
            ui.close()


if __name__ == '__main__':
    main()
