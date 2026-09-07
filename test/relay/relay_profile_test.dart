import 'dart:convert';
import 'dart:io';
import 'package:flutter_test/flutter_test.dart';
import 'package:hiddify/features/relay/relay_profile.dart';

void main() {
  final source = File('test/relay/profile.json').readAsStringSync();
  test('server-generated complete profile is accepted without conversion', () {
    expect(RelayProfile.validate(source)['outbounds'], hasLength(6));
    final reordered = (jsonDecode(source) as Map<String, dynamic>);
    expect(RelayProfile.validate(jsonEncode(reordered))['route']['final'], 'relay-select');
  });
  test('single-hop, direct, altered DNS and insecure TLS are rejected', () {
    for (final mutation in <void Function(Map<String, dynamic>)>[
      (c) => c['outbounds'][2].remove('detour'),
      (c) => c['outbounds'][2].remove('multiplex'),
      (c) => c['outbounds'][0]['outbounds'].add('direct'),
      (c) => c['outbounds'].add({'type': 'direct', 'tag': 'direct'}),
      (c) => c['outbounds'][3]['tls']['insecure'] = true,
      (c) => c['dns']['servers'][0]['detour'] = 'entry-a',
      (c) => c['route']['rules'] = [],
      (c) => c['outbounds'][1]['interval'] = '5m',
    ]) {
      final c = jsonDecode(source) as Map<String, dynamic>;
      mutation(c);
      expect(() => RelayProfile.validate(jsonEncode(c)), throwsA(anything));
    }
  });
  test('failed update leaves last correct file unchanged; valid replacement works', () async {
    final dir = await Directory.systemTemp.createTemp('relay-profile-test-');
    addTearDown(() => dir.delete(recursive: true));
    final good = File('${dir.path}/profile.json');
    final pending = File('${dir.path}/profile.pending');
    await good.writeAsString(source);
    await pending.writeAsString('{broken');
    await expectLater(RelayProfile.install(good.path, pending.path), throwsA(anything));
    expect(await good.readAsString(), source);
    final updated = source.replaceAll('192.0.2.10', '192.0.2.20');
    await pending.writeAsString(updated);
    await RelayProfile.install(good.path, pending.path);
    expect(await good.readAsString(), updated);
    expect(await pending.exists(), isFalse);
  });
  test('personal links reject redirects, userinfo and short tokens', () {
    final token = 'A' * 43;
    expect(RelayProfile.subscriptionUri('https://pilot.example/sub/$token').scheme, 'https');
    for (final link in [
      'http://pilot.example/sub/$token',
      'https://user:pass@pilot.example/sub/$token',
      'https://pilot.example/sub/short',
      'https://pilot.example/sub/$token?redirect=1',
    ]) {
      expect(() => RelayProfile.subscriptionUri(link), throwsFormatException);
    }
  });
}
