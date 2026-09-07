import importlib.machinery
import importlib.util
import io
import json
from pathlib import Path
import sys
import tempfile
import types
import unittest
from unittest.mock import patch

# Windows can exercise the state machine; real flock/systemd run on Linux.
if sys.platform == 'win32':
    sys.modules['fcntl'] = types.SimpleNamespace(LOCK_EX=2,flock=lambda *_:None)
loader=importlib.machinery.SourceFileLoader('relay_node',str(Path(__file__).parents[1]/'relaypilot-node'))
spec=importlib.util.spec_from_loader(loader.name,loader)
node=importlib.util.module_from_spec(spec)
loader.exec_module(node)

class NodeTest(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root=Path(self.tmp.name)
        self.calls=[]
        self.fail_check=False
        self.fail_restart=False
        if sys.platform == 'win32':
            p=patch.object(node.os,'fchmod',create=True);p.start();self.addCleanup(p.stop)
        patches=[patch.object(node,'ROOT',self.root),patch.object(node,'STATE',self.root/'state.json'),
                 patch.object(node,'CONFIG',self.root/'config.json'),patch.object(node.os,'geteuid',return_value=0,create=True),
                 patch.object(node.time,'time',return_value=1000),patch.object(node,'call',side_effect=self.call)]
        for p in patches:p.start();self.addCleanup(p.stop)
    def call(self,*args):
        self.calls.append(args)
        return not (self.fail_check and args[0]==node.BIN or self.fail_restart and 'restart' in args)
    def apply(self,generation,credentials='new',deadline=1060):
        body=json.dumps({'generation':generation,'valid_until':deadline,'config':{'users':[credentials]}}).encode()
        with patch.object(sys,'argv',['helper','apply']),patch.object(sys,'stdin',types.SimpleNamespace(buffer=io.BytesIO(body))):
            return node.main()
    def test_old_snapshot_cannot_resurrect_revoked_access(self):
        self.assertEqual(self.apply(20,'current'),0)
        self.assertEqual(self.apply(19,'revoked'),3)
        self.assertIn('current',node.CONFIG.read_text())
        self.assertNotIn('revoked',node.CONFIG.read_text())
    def test_failed_validation_keeps_last_config_and_generation(self):
        self.apply(1)
        self.fail_check=True
        self.assertEqual(self.apply(2,'bad'),4)
        self.assertEqual(json.loads(node.STATE.read_text())['generation'],1)
        self.assertIn('new',node.CONFIG.read_text())
    def test_unchanged_config_renews_without_restart(self):
        self.apply(1);self.calls.clear()
        self.assertEqual(self.apply(2),0)
        self.assertFalse(any('restart' in call for call in self.calls))
    def test_failed_restart_stops_and_never_restores_previous_keys(self):
        self.apply(1,'old');self.fail_restart=True
        self.assertEqual(self.apply(2,'revoked-all'),5)
        self.assertIn(('systemctl','stop',node.UNIT),self.calls)
        self.assertIn('revoked-all',node.CONFIG.read_text())
    def test_expired_lease_stops_transport(self):
        self.apply(1)
        with patch.object(node.time,'time',return_value=1061),patch.object(sys,'argv',['helper','guard']):
            self.assertEqual(node.main(),1)
        self.assertIn(('systemctl','stop',node.UNIT),self.calls)
        self.assertEqual(self.apply(2,deadline=999),2)

if __name__=='__main__': unittest.main()
