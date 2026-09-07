import 'dart:convert';
import 'dart:io';

/// The pilot deliberately accepts only its closed, two-hop configuration format.
/// This is also used for offline imports, updates and every core start/restart.
abstract final class RelayProfile {
  static const maxBytes = 1024 * 1024;
  static const modes = ['relay-auto', 'route-a', 'route-b'];

  static Uri subscriptionUri(String value) {
    final u = Uri.tryParse(value.trim());
    if (u == null ||
        u.scheme != 'https' ||
        u.host.isEmpty ||
        u.userInfo.isNotEmpty ||
        u.hasQuery ||
        !RegExp(r'^/sub/[A-Za-z0-9_-]{43}$').hasMatch(u.path)) {
      throw const FormatException('Нужна персональная HTTPS-ссылка Relay Pilot.');
    }
    return u;
  }

  static Map<String, dynamic> validate(String content) {
    if (utf8.encode(content).length > maxBytes) _bad();
    final c = _map(jsonDecode(content));
    _keys(c, {'log', 'dns', 'inbounds', 'outbounds', 'route'});
    if (jsonEncode(c['log']) != '{"disabled":true}') _bad();
    final inbound = (c['inbounds'] as List).single;
    final tun = _map(inbound);
    _keys(tun, {'type', 'tag', 'address', 'mtu', 'auto_route', 'strict_route', 'stack'});
    if (tun['type'] != 'tun' ||
        tun['tag'] != 'relay-tun' ||
        tun['auto_route'] != true ||
        tun['strict_route'] != true ||
        tun['stack'] != 'mixed' ||
        tun['mtu'] != 1400 ||
        jsonEncode(tun['address']) != '["172.19.0.1/30","fdfe:dcba:9876::1/126"]')
      _bad();
    final out = <String, Map<String, dynamic>>{};
    for (final value in c['outbounds'] as List) {
      final o = _map(value);
      final tag = o['tag'] as String;
      if (out.containsKey(tag)) _bad();
      out[tag] = o;
    }
    if (out.length != 6 ||
        !out.keys.toSet().containsAll({'relay-select', 'relay-auto', 'route-a', 'route-b', 'entry-a', 'entry-b'}))
      _bad();
    final select = out['relay-select']!;
    _keys(select, {'type', 'tag', 'outbounds', 'default', 'interrupt_exist_connections'});
    if (select['type'] != 'selector' ||
        select['default'] != 'relay-auto' ||
        select['interrupt_exist_connections'] != true ||
        jsonEncode(select['outbounds']) != jsonEncode(modes))
      _bad();
    final auto = out['relay-auto']!;
    _keys(auto, {
      'type',
      'tag',
      'outbounds',
      'url',
      'interval',
      'tolerance',
      'idle_timeout',
      'interrupt_exist_connections',
    });
    final probe = Uri.tryParse(auto['url'] as String);
    if (auto['type'] != 'urltest' ||
        auto['interval'] != '15s' ||
        auto['tolerance'] != 100 ||
        auto['idle_timeout'] != '24h' ||
        auto['interrupt_exist_connections'] != true ||
        probe?.scheme != 'https' ||
        probe!.host.isEmpty ||
        probe.userInfo.isNotEmpty ||
        jsonEncode(auto['outbounds']) != '["route-a","route-b"]')
      _bad();
    for (final tag in ['route-a', 'route-b', 'entry-a', 'entry-b']) {
      final o = out[tag]!;
      final isEntry = tag.startsWith('entry');
      final isB = tag == 'entry-b';
      _keys(o, {
        'type',
        'tag',
        'server',
        'server_port',
        'tls',
        if (isB) 'password' else 'uuid',
        if (!isEntry) ...['detour', 'packet_encoding', 'multiplex'],
      });
      if (o['type'] != (isB ? 'hysteria2' : 'vless') ||
          InternetAddress.tryParse(o['server'] as String) == null ||
          o['server_port'] is! int ||
          (o['server_port'] as int) < 1 ||
          (o['server_port'] as int) > 65535)
        _bad();
      final credential = o[isB ? 'password' : 'uuid'] as String;
      if (isB
          ? !RegExp(r'^[A-Za-z0-9_-]{43}$').hasMatch(credential)
          : !RegExp(r'^[a-fA-F0-9]{8}-(?:[a-fA-F0-9]{4}-){3}[a-fA-F0-9]{12}$').hasMatch(credential))
        _bad();
      if (!isEntry && (o['detour'] != (tag == 'route-a' ? 'entry-a' : 'entry-b') || o['packet_encoding'] != 'xudp'))
        _bad();
      if (!isEntry) {
        final mux = _map(o['multiplex']);
        _keys(mux, {'enabled', 'protocol'});
        if (mux['enabled'] != true || mux['protocol'] != 'smux') _bad();
      }
      final tls = _map(o['tls']);
      _keys(tls, {
        'enabled',
        'server_name',
        if (tag == 'entry-a') ...['utls', 'reality'],
      });
      if (tls['enabled'] != true || (tls['server_name'] as String).isEmpty) _bad();
      if (tag == 'entry-a') {
        final reality = _map(tls['reality']);
        _keys(reality, {'enabled', 'public_key', 'short_id'});
        if (reality['enabled'] != true ||
            !RegExp(r'^[A-Za-z0-9_-]{43}$').hasMatch(reality['public_key'] as String) ||
            !RegExp(r'^(?:[a-fA-F0-9]{2}){1,8}$').hasMatch(reality['short_id'] as String))
          _bad();
        final utls = _map(tls['utls']);
        _keys(utls, {'enabled', 'fingerprint'});
        if (utls['enabled'] != true || utls['fingerprint'] != 'chrome') _bad();
      }
    }
    final a = Map<String, dynamic>.from(out['route-a']!)
      ..remove('tag')
      ..remove('detour');
    final b = Map<String, dynamic>.from(out['route-b']!)
      ..remove('tag')
      ..remove('detour');
    if (jsonEncode(a) != jsonEncode(b) ||
        out['entry-a']!['server'] == out['entry-b']!['server'] ||
        out['entry-a']!['server'] == a['server'] ||
        out['entry-b']!['server'] == a['server'])
      _bad();
    final route = _map(c['route']);
    _keys(route, {'rules', 'final', 'auto_detect_interface', 'default_domain_resolver'});
    final rules = route['rules'] as List;
    if (rules.length != 2 || jsonEncode(rules.first) != '{"action":"sniff"}') _bad();
    final rule = _map(rules.last);
    _keys(rule, {'protocol', 'action'});
    if (rule['protocol'] != 'dns' ||
        rule['action'] != 'hijack-dns' ||
        route['final'] != 'relay-select' ||
        route['auto_detect_interface'] != true ||
        route['default_domain_resolver'] != 'relay-dns')
      _bad();
    final dns = _map(c['dns']);
    _keys(dns, {'servers', 'final', 'strategy'});
    final resolver = _map((dns['servers'] as List).single);
    _keys(resolver, {'type', 'tag', 'server', 'server_port', 'path', 'tls', 'detour'});
    if (dns['final'] != 'relay-dns' ||
        dns['strategy'] != 'prefer_ipv4' ||
        resolver['type'] != 'https' ||
        resolver['tag'] != 'relay-dns' ||
        InternetAddress.tryParse(resolver['server'] as String) == null ||
        resolver['server_port'] != 443 ||
        resolver['path'] != '/dns-query' ||
        resolver['detour'] != 'relay-select' ||
        jsonEncode(resolver['tls']) != '{"enabled":true}')
      _bad();
    return c;
  }

  static Future<void> validateFile(String path) async {
    final f = File(path);
    if (await f.length() > maxBytes) _bad();
    validate(await f.readAsString());
  }

  /// Staging is on the same filesystem. Invalid updates leave the old file intact.
  static Future<void> install(String destination, String staging) async {
    await validateFile(staging);
    await File(staging).rename(destination);
  }

  static Map<String, dynamic> _map(dynamic v) => (v as Map).cast<String, dynamic>();
  static void _keys(Map<String, dynamic> m, Set<String> keys) {
    if (m.length != keys.length || !m.keys.every(keys.contains)) _bad();
  }

  static Never _bad() => throw const FormatException('Профиль Relay Pilot повреждён или несовместим.');
}
