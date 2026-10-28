#!/usr/bin/env python3
"""Offline safety tests. Every AWS call is replaced with an in-memory fake."""
import datetime as dt
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from contextlib import redirect_stdout
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
SCRATCH = ROOT / '.tmp'
SCRATCH.mkdir(exist_ok=True)


def module(name, path):
    spec = importlib.util.spec_from_file_location(name, ROOT / path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


arm = module('arm_teardown', 'scripts/sre-lab/arm-teardown.py')
worker = module('teardown_worker', 'scripts/sre-lab/teardown/worker.py')
login = module('lab_login', 'scripts/sre-lab/login.py')


class TeardownTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=SCRATCH)
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0)
        self.env = {
            'TF_VAR_alert_email': 'review@example.invalid',
            'TF_VAR_budget_alert_email': 'review@example.invalid',
            'TF_VAR_expires_after': (self.now.date() + dt.timedelta(days=1)).isoformat(),
            'TF_VAR_foundation_state_bucket': 'mycaresignals-sre-lab-tfstate-266877689034-eu-west-2',
            'TF_VAR_terraform_lock_table': 'mycaresignals-sre-lab-terraform-locks',
            'TF_VAR_operator_cidrs': '["203.0.113.10/32"]',
            'GITHUB_ACTIONS': 'true', 'GITHUB_REPOSITORY': 'victorex001/mycaresignals-platform-sre-lab',
            'GITHUB_REF': 'refs/heads/main', 'CONFIRM_SRE_LAB_LAUNCH': arm.CONFIRMATION,
            'CODEBUILD_BUILD_ID': 'mycaresignals-sre-lab-teardown:current',
        }
        self.lab = self.directory / 'lab'
        for stack in ('foundation', 'cluster'):
            directory = self.lab / stack
            directory.mkdir(parents=True)
            (directory / 'main.tf').write_text('terraform {}\n')
            (directory / '.terraform.lock.hcl').write_text('# fixture lock\n')
            (directory / 'terraform.tfstate').write_text('must not enter bundle')
            (directory / 'terraform.tfvars').write_text('must not enter bundle')
        (self.lab / 'cluster/eks.tf').write_text('# fixture cluster\n')
        lab_patch = patch.object(arm, 'LAB', self.lab)
        lab_patch.start()
        self.addCleanup(lab_patch.stop)
        self.environment = patch.dict(os.environ, self.env, clear=True)
        self.environment.start()
        self.addCleanup(self.environment.stop)
        self.config = arm.parameters(self.now, 'run-test')
        self.archive = self.directory / 'source.zip'
        self.calls = []
        self.payload = None
        self.running = []
        self.prior = None
        self.account = arm.ACCOUNT
        self.version = 'immutable-version'
        self.execution_status = 'RUNNING'
        self.nonempty_state = False

    def cloud(self, service, operation, *args):
        self.calls.append((service, operation, args))
        if (service, operation) == ('sts', 'get-caller-identity'):
            return {'Account': self.account}
        if operation == 'describe-state-machine':
            return {'type': 'STANDARD', 'status': 'ACTIVE'}
        if operation == 'list-executions':
            return {'executions': self.running}
        if operation == 'describe-execution':
            return {'status': self.execution_status, 'input': json.dumps(self.prior or self.payload)}
        if operation == 'list-objects-v2':
            return {'Contents': [{'Key': 'mycaresignals/sre-lab/foundation/terraform.tfstate'}]} if self.nonempty_state else {}
        if operation == 'get-object':
            Path(args[-1]).write_text(json.dumps({'resources': [{'mode': 'managed', 'instances': [{}]}]}))
            return {}
        if operation == 'list-subscriptions-by-topic':
            return {'Subscriptions': [{'Protocol': 'email', 'SubscriptionArn': 'arn:aws:sns:eu-west-2:266877689034:confirmed'}]}
        if operation == 'get-bucket-versioning':
            return {'Status': 'Enabled'}
        if operation == 'get-public-access-block':
            return {'PublicAccessBlockConfiguration': dict.fromkeys(['BlockPublicAcls', 'IgnorePublicAcls', 'BlockPublicPolicy', 'RestrictPublicBuckets'], True)}
        if operation == 'put-object':
            return {'VersionId': self.version}
        if operation == 'start-execution':
            self.payload = json.loads(args[args.index('--input') + 1])
            return {'executionArn': 'arn:aws:states:eu-west-2:266877689034:execution:mycaresignals-sre-lab-teardown:run-test'}
        raise AssertionError('Unexpected AWS call in test')

    def invoke_arm(self, fingerprint='fingerprint'):
        with patch.object(arm, 'aws', side_effect=self.cloud), redirect_stdout(io.StringIO()):
            arm.arm(self.config, self.archive, 'a' * 64, fingerprint)

    def test_production_identity_blocks_every_write(self):
        self.account = '121638211660'
        with self.assertRaisesRegex(RuntimeError, 'account'):
            self.invoke_arm()
        self.assertEqual([x[:2] for x in self.calls], [('sts', 'get-caller-identity')])

    def test_arming_requires_explicit_main_approval(self):
        os.environ['CONFIRM_SRE_LAB_LAUNCH'] = 'wrong'
        with self.assertRaisesRegex(RuntimeError, 'approved'):
            self.invoke_arm()
        self.assertEqual(self.calls, [])

    def test_verified_cloud_timer_is_created_with_immutable_source(self):
        self.invoke_arm()
        self.assertEqual(self.payload['source_version'], 'immutable-version')
        self.assertEqual(self.payload['teardown_at'], self.config['teardown_at'])
        self.assertEqual([x[1] for x in self.calls if x[1] in ['put-object', 'start-execution']], ['put-object', 'start-execution'])

    def test_unversioned_source_never_starts_timer(self):
        self.version = 'null'
        with self.assertRaisesRegex(RuntimeError, 'immutable'):
            self.invoke_arm()
        self.assertNotIn('start-execution', [x[1] for x in self.calls])

    def test_unconfirmed_timer_is_not_accepted(self):
        self.execution_status = 'FAILED'
        with self.assertRaisesRegex(RuntimeError, 'acceptance'):
            self.invoke_arm()

    def test_existing_timer_is_reused_without_extending_deadline(self):
        self.running = [{'executionArn': 'existing'}]
        due = (self.now + dt.timedelta(hours=3)).isoformat()
        self.prior = {'fingerprint': 'fingerprint', 'teardown_at': due}
        self.invoke_arm()
        self.assertNotIn('put-object', [x[1] for x in self.calls])
        self.assertNotIn('start-execution', [x[1] for x in self.calls])
        self.assertEqual(self.prior['teardown_at'], due)

    def test_changed_configuration_cannot_reuse_approval(self):
        self.running = [{'executionArn': 'existing'}]
        self.prior = {'fingerprint': 'different', 'teardown_at': self.config['teardown_at']}
        with self.assertRaisesRegex(RuntimeError, 'different'):
            self.invoke_arm()

    def test_short_remaining_window_blocks_apply(self):
        self.running = [{'executionArn': 'existing'}]
        self.prior = {'fingerprint': 'fingerprint', 'teardown_at': (self.now + dt.timedelta(hours=1)).isoformat()}
        with self.assertRaisesRegex(RuntimeError, 'insufficient'):
            self.invoke_arm()

    def test_previous_state_cannot_receive_a_new_clock(self):
        self.nonempty_state = True
        with self.assertRaisesRegex(RuntimeError, 'not empty'):
            self.invoke_arm()
        self.assertNotIn('start-execution', [x[1] for x in self.calls])
        self.assertFalse(list(self.directory.glob('*.prior-state.json')))

    def test_bundle_allowlist_excludes_state_vars_and_caches(self):
        files, digest, fingerprint = arm.snapshot(self.config)
        self.assertIn('cluster/.terraform.lock.hcl', files)
        self.assertIn('foundation/main.tf', files)
        self.assertFalse(any('/.terraform/' in x or x.endswith(('.tfstate', '.tfvars')) for x in files))
        arm.write_bundle(self.archive, files)
        self.assertEqual(self.archive.stat().st_mode & 0o777, 0o600)
        self.assertEqual(len(digest), 64)

    def test_manifest_tampering_is_rejected(self):
        config = arm.parameters(self.now - dt.timedelta(hours=5), 'run-test')
        files, digest, _ = arm.snapshot(config)
        for name, content in files.items():
            path = self.directory / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(content)
        with patch.object(worker, 'ROOT', self.directory):
            self.assertEqual(worker.verify_bundle(digest, 'run-test')['account'], arm.ACCOUNT)
            (self.directory / 'cluster/eks.tf').write_text('changed after approval')
            with self.assertRaisesRegex(RuntimeError, 'Pinned source'):
                worker.verify_bundle(digest, 'run-test')

    def test_future_deadline_is_not_deleted_early(self):
        files, digest, _ = arm.snapshot(self.config)
        for name, content in files.items():
            path = self.directory / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(content)
        with patch.object(worker, 'ROOT', self.directory), self.assertRaisesRegex(RuntimeError, 'not expired'):
            worker.verify_bundle(digest, 'run-test')

    def test_create_update_and_replacement_are_forbidden(self):
        for actions in [['create'], ['update'], ['delete', 'create']]:
            with self.subTest(actions=actions), self.assertRaisesRegex(RuntimeError, 'create/update'):
                worker.require_delete_only({'resource_changes': [{'mode':'managed','change':{'actions':actions}}]})

    def test_foreign_plan_arn_is_rejected(self):
        plan = {'resource_changes': [{'mode':'managed','provider_name':'registry.terraform.io/hashicorp/aws','change':{'actions':['delete'],'before':{'arn':'arn:aws:rds:eu-west-2:121638211660:db:production'}}}]}
        with self.assertRaisesRegex(RuntimeError, 'foreign'):
            worker.require_delete_only(plan)

    def test_local_worker_execution_is_blocked(self):
        with patch.object(worker, 'aws') as cloud, self.assertRaisesRegex(RuntimeError, 'CodeBuild'):
            worker.main()
        cloud.assert_not_called()

    def test_cluster_failure_prevents_network_deletion(self):
        os.environ['CODEBUILD_BUILD_ARN'] = 'arn:aws:codebuild:eu-west-2:266877689034:build/mycaresignals-sre-lab-teardown:test'
        with patch.object(worker, 'aws', return_value={'Account':arm.ACCOUNT}), patch.object(worker, 'verify_bundle', return_value=self.config), patch.object(worker,'worker_history',return_value=[]), patch.object(worker,'install_terraform',return_value='fake-terraform'), patch.object(worker,'destroy_stack',side_effect=RuntimeError('cluster deletion failed')) as destroy:
            with self.assertRaises(RuntimeError):
                worker.main()
            self.assertEqual(destroy.call_args.args[2], 'cluster')
            self.assertEqual(destroy.call_count, 1)

    def lock_cloud(self, active=False, owner='runner@github'):
        self.config['teardown_at'] = (self.now-dt.timedelta(minutes=1)).isoformat()
        lock = {'ID':'00000000-0000-0000-0000-000000000000','Path': self.config['state_bucket']+'/mycaresignals/sre-lab/cluster/terraform.tfstate','Who':owner,'Created':(self.now-dt.timedelta(hours=6)).isoformat()}
        calls = []
        def cloud(service, operation, *args):
            calls.append((service, operation, args))
            if operation == 'get-item':
                return {'Item': {'Info': {'S': json.dumps(lock)}}}
            if operation == 'batch-get-builds':
                return {'builds': [{'buildStatus': 'IN_PROGRESS' if active else 'TIMED_OUT'}]}
            if operation == 'delete-item':
                return {}
            raise AssertionError('Unexpected lock AWS call')
        return cloud, calls

    def test_active_github_lock_is_preserved(self):
        cloud, calls = self.lock_cloud()
        with patch.object(worker,'aws',side_effect=cloud),patch.object(worker,'command',return_value=b'{"workflow_runs":[{}]}'), self.assertRaisesRegex(RuntimeError,'active'):
            worker.recover_backend_lock(self.config,'cluster',[])
        self.assertNotIn('delete-item',[x[1] for x in calls])

    def test_inactive_github_lock_is_conditionally_removed(self):
        cloud,calls=self.lock_cloud()
        with patch.object(worker,'aws',side_effect=cloud),patch.object(worker,'command',return_value=b'{"workflow_runs":[]}'), redirect_stdout(io.StringIO()):
            worker.recover_backend_lock(self.config,'cluster',[])
        deletion=next(x for x in calls if x[1]=='delete-item')
        self.assertIn('--condition-expression',deletion[2])
        self.assertIn('--expression-attribute-values',deletion[2])

    def test_active_or_unknown_worker_lock_is_preserved(self):
        history=[{'who':'root@prior','build_id':'mycaresignals-sre-lab-teardown:prior'}]
        cloud,calls=self.lock_cloud(active=True,owner='root@prior')
        with patch.object(worker,'aws',side_effect=cloud),self.assertRaisesRegex(RuntimeError,'active'):
            worker.recover_backend_lock(self.config,'cluster',history)
        self.assertNotIn('delete-item',[x[1] for x in calls])

    def test_stopped_prior_worker_lock_can_be_recovered(self):
        history=[{'who':'root@prior','build_id':'mycaresignals-sre-lab-teardown:prior'}]
        cloud,calls=self.lock_cloud(owner='root@prior')
        with patch.object(worker,'aws',side_effect=cloud),redirect_stdout(io.StringIO()):
            worker.recover_backend_lock(self.config,'cluster',history)
        self.assertIn('delete-item',[x[1] for x in calls])

    def test_cluster_security_group_with_interfaces_is_preserved(self):
        calls = []
        def cloud(service, operation, *args):
            calls.append(operation)
            if operation == 'list-clusters': return {'clusters': []}
            if operation == 'describe-vpcs': return {'Vpcs': [{'VpcId': 'vpc-lab'}]}
            if operation == 'describe-security-groups': return {'SecurityGroups': [{
                'OwnerId':arm.ACCOUNT,'VpcId':'vpc-lab','GroupId':'sg-lab',
                'GroupName':'eks-cluster-sg-mycaresignals-sre-lab-eks-123',
                'Tags':[{'Key':'kubernetes.io/cluster/mycaresignals-sre-lab-eks','Value':'owned'}]}]}
            if operation == 'describe-network-interfaces': return {'NetworkInterfaces': [{}]}
            raise AssertionError('Unexpected deletion')
        with patch.object(worker,'aws',side_effect=cloud),self.assertRaisesRegex(RuntimeError,'interfaces'):
            worker.cleanup_cluster_security_group()
        self.assertNotIn('delete-security-group', calls)

    def test_live_cluster_blocks_security_group_cleanup(self):
        with patch.object(worker,'aws',return_value={'clusters':['mycaresignals-sre-lab-eks']}) as cloud,self.assertRaisesRegex(RuntimeError,'Cluster still exists'):
            worker.cleanup_cluster_security_group()
        self.assertEqual(cloud.call_count,1)

    def test_login_only_changes_the_named_profile(self):
        path=self.directory/'credentials'
        original='[default]\naws_access_key_id = old-default\n\n[mycaresignals-plan]\naws_access_key_id = untouched\n'
        path.write_text(original)
        login.replace_profile(path,'sre-lab','aws_access_key_id = local-test\n')
        self.assertTrue(path.read_text().startswith(original))
        first=path.read_text()
        login.replace_profile(path,'sre-lab','aws_access_key_id = second-test\n')
        self.assertTrue(path.read_text().startswith(original))
        self.assertNotIn('local-test',path.read_text())


if __name__ == '__main__':
    unittest.main()
