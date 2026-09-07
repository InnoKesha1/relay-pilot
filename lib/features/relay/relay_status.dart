import 'package:hiddify/features/connection/notifier/connection_notifier.dart';
import 'package:hiddify/hiddifycore/hiddify_core_service_provider.dart';
import 'package:hiddify/hiddifycore/generated/v2/hcommon/common.pb.dart';
import 'package:hiddify/hiddifycore/generated/v2/hcore/hcore.pb.dart';
import 'package:hiddify/hiddifycore/init_signal.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

final relayGroupsProvider = StreamProvider<List<OutboundGroup>>((ref) async* {
  ref.watch(coreRestartSignalProvider);
  if (!await ref.watch(serviceRunningProvider.future)) {
    yield [];
    return;
  }
  final service = ref.watch(hiddifyCoreServiceProvider);
  yield* service.core.bgClient.outboundsInfo(Empty()).map((v) => v.items.toList());
});

String relayRouteLabel(String tag) => switch (tag) {
  'route-a' => 'Маршрут A',
  'route-b' => 'Маршрут B',
  'relay-auto' => 'Авто',
  _ => 'Ожидание маршрута',
};
