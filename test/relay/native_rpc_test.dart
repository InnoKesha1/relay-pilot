import 'dart:io';
import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:hiddify/hiddifycore/core_interface/core_interface_desktop.dart';
import 'package:hiddify/hiddifycore/generated/v2/hcommon/common.pb.dart';
import 'package:hiddify/hiddifycore/generated/v2/hello/hello.pb.dart';
import 'package:hiddify/hiddifycore/generated/v2/hello/hello_service.pbgrpc.dart';

void main() {
  test(
    'native core accepts our mTLS client and rejects unauthenticated RPC',
    () async {
      final originalDirectory = Directory.current;
      final temp = await Directory.systemTemp.createTemp('relay-rpc-');
      final core = CoreInterfaceDesktop();
      final anonymous = ClientChannel(
        '127.0.0.1',
        port: CoreInterfaceDesktop.port,
        options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
      );
      // Test-only: trust this loopback server but present no client identity.
      final noIdentity = ClientChannel(
        '127.0.0.1',
        port: CoreInterfaceDesktop.port,
        options: ChannelOptions(credentials: ChannelCredentials.secure(onBadCertificate: (_, __) => true)),
      );
      try {
        final result = await core.setup((baseDir: temp, workingDir: temp, tempDir: temp), false, 1);
        expect(result, isEmpty);
        await core.fgClient.getSystemInfo(Empty(), options: CallOptions(timeout: const Duration(seconds: 5)));
        await expectLater(
          HelloClient(anonymous).sayHello(
            HelloRequest(name: 'unauthenticated'),
            options: CallOptions(timeout: const Duration(seconds: 3)),
          ),
          throwsA(isA<GrpcError>()),
        );
        await expectLater(
          HelloClient(noIdentity).sayHello(
            HelloRequest(name: 'no-client-certificate'),
            options: CallOptions(timeout: const Duration(seconds: 3)),
          ),
          throwsA(isA<GrpcError>()),
        );
        // Reinitialization must retain authenticated access without attaching to another app.
        expect(await core.setup((baseDir: temp, workingDir: temp, tempDir: temp), false, 1), isEmpty);
      } finally {
        await anonymous.terminate();
        await noIdentity.terminate();
        await core.dispose();
        Directory.current = originalDirectory;
        // Native SQLite retains file handles until process exit on Windows.
      }
    },
    skip: Platform.environment['RELAY_NATIVE_RPC_TEST'] != '1',
    timeout: const Timeout(Duration(seconds: 40)),
  );
}
