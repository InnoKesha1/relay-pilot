import 'dart:async';
import 'dart:io';
import 'package:flutter/services.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hiddify/core/app_info/app_info_provider.dart';
import 'package:hiddify/core/model/app_info_entity.dart';
import 'package:hiddify/features/connection/notifier/connection_notifier.dart';
import 'package:hiddify/features/connection/model/connection_status.dart';
import 'package:hiddify/features/profile/notifier/active_profile_notifier.dart';
import 'package:hiddify/features/profile/model/profile_entity.dart';
import 'package:hiddify/features/relay/relay_home_page.dart';
import 'package:hiddify/features/relay/relay_status.dart';

class EmptyProfile extends ActiveProfile {
  @override
  Stream<ProfileEntity?> build() => Stream.value(null);
}

class OfflineConnection extends ConnectionNotifier {
  @override
  Stream<ConnectionStatus> build() => Stream.value(const ConnectionStatus.disconnected());
}

class NoAppInfo extends AppInfo {
  @override
  Future<AppInfoEntity> build() => Completer<AppInfoEntity>().future;
}

void main() {
  testWidgets('first launch offers personal access and shows no false connection', (tester) async {
    final capture = Platform.environment['RELAY_CAPTURE_UI'] == '1';
    if (capture) {
      await tester.runAsync(() async {
        final bytes = File(Platform.environment['RELAY_CAPTURE_FONT']!).readAsBytesSync();
        final font = FontLoader('RelayCapture')..addFont(Future.value(ByteData.sublistView(bytes)));
        await font.load();
        final icons = File(Platform.environment['RELAY_CAPTURE_ICONS']!).readAsBytesSync();
        await (FontLoader('MaterialIcons')..addFont(Future.value(ByteData.sublistView(icons)))).load();
      });
    }
    await tester.binding.setSurfaceSize(const Size(430, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          activeProfileProvider.overrideWith(EmptyProfile.new),
          connectionNotifierProvider.overrideWith(OfflineConnection.new),
          appInfoProvider.overrideWith(NoAppInfo.new),
          relayGroupsProvider.overrideWith((ref) => Stream.value([])),
        ],
        child: MaterialApp(
          theme: ThemeData(
            useMaterial3: true,
            brightness: Brightness.dark,
            colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF168C83), brightness: Brightness.dark),
            fontFamily: capture ? 'RelayCapture' : null,
          ),
          home: const HomePage(),
        ),
      ),
    );
    await tester.pump();
    expect(find.text('Добавить персональный доступ'), findsOneWidget);
    expect(find.text('Соединение выключено'), findsOneWidget);
    expect(find.text('Соединение работает'), findsNothing);
    expect(find.text('Маршрут A'), findsOneWidget);
    expect(tester.takeException(), isNull);
    if (capture) await expectLater(find.byType(HomePage), matchesGoldenFile('../../docs/screenshots/home.png'));
  });
}
