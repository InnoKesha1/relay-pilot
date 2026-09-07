import 'package:flutter/material.dart';
import 'package:hiddify/features/relay/relay_about.dart';
import 'package:hiddify/features/profile/model/profile_entity.dart';
import 'package:hiddify/core/app_info/app_info_provider.dart';
import 'package:hiddify/core/router/bottom_sheets/bottom_sheets_notifier.dart';
import 'package:hiddify/features/connection/notifier/connection_notifier.dart';
import 'package:hiddify/features/profile/notifier/active_profile_notifier.dart';
import 'package:hiddify/features/profile/notifier/profile_notifier.dart';
import 'package:hiddify/features/relay/relay_profile.dart';
import 'package:hiddify/features/relay/relay_status.dart';
import 'package:hiddify/hiddifycore/hiddify_core_service_provider.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class HomePage extends ConsumerWidget {
  const HomePage({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final profile = ref.watch(activeProfileProvider).valueOrNull;
    final connection = ref.watch(connectionNotifierProvider);
    final connected = connection.valueOrNull?.isConnected ?? false;
    final transitioning = connection.valueOrNull?.isSwitching ?? false;
    final groups = ref.watch(relayGroupsProvider).valueOrNull ?? [];
    final root = groups.where((g) => g.tag == 'relay-select').firstOrNull;
    final auto = groups.where((g) => g.tag == 'relay-auto').firstOrNull;
    final mode = RelayProfile.modes.contains(root?.selected) ? root!.selected : 'relay-auto';
    final active = mode == 'relay-auto' ? auto?.selected ?? '' : mode;
    final info = groups.expand((g) => g.items).where((o) => o.tag == active).firstOrNull;
    final delay = info?.urlTestDelay ?? 0;
    final ready = connected && delay > 0 && delay <= 65000;
    void add() => ref.read(bottomSheetsNotifierProvider.notifier).showAddProfile();
    return Scaffold(
      appBar: AppBar(
        title: const Text('RELAY PILOT'),
        actions: [
          IconButton(
            onPressed: () => showRelayAbout(context),
            tooltip: 'О приложении',
            icon: const Icon(Icons.info_outline),
          ),
          IconButton(onPressed: add, tooltip: 'Добавить доступ', icon: const Icon(Icons.add_link_rounded)),
          const SizedBox(width: 12),
        ],
      ),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 640),
          child: ListView(
            padding: const EdgeInsets.all(24),
            children: [
              Text(
                'Свой путь\nк интернету.',
                style: theme.textTheme.displaySmall?.copyWith(fontWeight: FontWeight.w700, height: 1.05),
              ),
              const SizedBox(height: 12),
              Text('Два узла в цепочке. Два маршрута на выбор.', style: theme.textTheme.bodyLarge),
              const SizedBox(height: 32),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Row(
                        children: [
                          Icon(
                            ready ? Icons.check_circle_rounded : Icons.circle_outlined,
                            color: theme.colorScheme.primary,
                          ),
                          const SizedBox(width: 10),
                          Expanded(
                            child: Text(
                              transitioning
                                  ? 'Подключаемся…'
                                  : ready
                                  ? 'Соединение работает'
                                  : connected
                                  ? 'Проверяем доступность маршрута'
                                  : 'Соединение выключено',
                              style: theme.textTheme.titleMedium,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 28),
                      FilledButton.icon(
                        style: FilledButton.styleFrom(padding: const EdgeInsets.symmetric(vertical: 22)),
                        onPressed: transitioning
                            ? null
                            : profile == null
                            ? add
                            : () => ref.read(connectionNotifierProvider.notifier).toggleConnection(),
                        icon: Icon(connected ? Icons.stop_rounded : Icons.power_settings_new_rounded),
                        label: Text(
                          profile == null
                              ? 'Добавить персональный доступ'
                              : connected
                              ? 'Отключить'
                              : 'Подключить',
                        ),
                      ),
                      const SizedBox(height: 22),
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Text(connected ? relayRouteLabel(active) : 'Маршрут ещё не выбран'),
                          Text(ready ? '$delay мс' : '—'),
                        ],
                      ),
                      const SizedBox(height: 22),
                      const Row(
                        children: [
                          Expanded(child: _Node(Icons.devices_rounded, 'Устройство')),
                          Icon(Icons.chevron_right_rounded),
                          Expanded(child: _Node(Icons.login_rounded, 'Вход')),
                          Icon(Icons.chevron_right_rounded),
                          Expanded(child: _Node(Icons.public_rounded, 'Выход')),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 24),
              Text('Режим подключения', style: theme.textTheme.titleMedium),
              const SizedBox(height: 12),
              SegmentedButton<String>(
                segments: RelayProfile.modes
                    .map((m) => ButtonSegment(value: m, label: Text(relayRouteLabel(m))))
                    .toList(),
                selected: {mode},
                onSelectionChanged: !connected
                    ? null
                    : (selection) async {
                        final result = await ref
                            .read(hiddifyCoreServiceProvider)
                            .selectOutbound('relay-select', selection.single)
                            .run();
                        result.fold((_) {
                          if (context.mounted)
                            ScaffoldMessenger.of(
                              context,
                            ).showSnackBar(const SnackBar(content: Text('Не удалось переключить маршрут.')));
                        }, (_) {});
                      },
              ),
              const SizedBox(height: 12),
              const Text(
                'В режиме «Авто» приложение проверяет оба пути и выбирает доступный. При смене пути приложения могут переподключиться.',
              ),
              const SizedBox(height: 24),
              if (profile is RemoteProfileEntity)
                ListTile(
                  contentPadding: EdgeInsets.zero,
                  leading: const Icon(Icons.key_outlined),
                  title: Text(profile.name),
                  subtitle: const Text('Персональный доступ · обновляется автоматически'),
                  trailing: IconButton(
                    icon: const Icon(Icons.refresh),
                    tooltip: 'Обновить доступ',
                    onPressed: () =>
                        ref.read(updateProfileNotifierProvider(profile.id).notifier).updateProfile(profile),
                  ),
                ),
              const Divider(height: 36),
              Text('Закрытый некоммерческий пилот', style: theme.textTheme.labelLarge),
              const SizedBox(height: 8),
              const Text('Работа при белых списках проверяется отдельно для каждой сети.'),
              const SizedBox(height: 18),
              const Text('На основе Hiddify и sing-box. Исходники и лицензия доступны в разделе «О приложении».'),
              const SizedBox(height: 8),
              const AppVersionLabel(),
            ],
          ),
        ),
      ),
    );
  }
}

class _Node extends StatelessWidget {
  const _Node(this.icon, this.label);
  final IconData icon;
  final String label;
  @override
  Widget build(BuildContext context) => Column(
    children: [
      Icon(icon, size: 25),
      const SizedBox(height: 8),
      Text(label, style: Theme.of(context).textTheme.labelSmall),
    ],
  );
}

class AppVersionLabel extends ConsumerWidget {
  const AppVersionLabel({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) =>
      Text(ref.watch(appInfoProvider).valueOrNull?.presentVersion ?? '', style: Theme.of(context).textTheme.labelSmall);
}
