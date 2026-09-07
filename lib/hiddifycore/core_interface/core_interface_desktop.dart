import 'dart:convert';
import 'dart:ffi';
import 'dart:io';
import 'dart:typed_data';
import 'package:ffi/ffi.dart';
import 'package:grpc/grpc.dart';
import 'package:hiddify/core/model/directories.dart';
import 'package:hiddify/gen/hiddify_core_generated_bindings.dart';
import 'package:hiddify/hiddifycore/core_interface/core_interface.dart';
import 'package:hiddify/hiddifycore/core_interface/mtls_channel_cred.dart';
import 'package:hiddify/hiddifycore/generated/v2/hcore/hcore.pb.dart';
import 'package:hiddify/hiddifycore/generated/v2/hcore/hcore_service.pbgrpc.dart';
import 'package:hiddify/hiddifycore/generated/v2/hello/hello.pb.dart';
import 'package:hiddify/hiddifycore/generated/v2/hello/hello_service.pbgrpc.dart';
import 'package:path/path.dart' as p;

class CoreInterfaceDesktop extends CoreInterface {
  static final HiddifyCoreNativeLibrary _box = HiddifyCoreNativeLibrary(
    DynamicLibrary.open(
      p.join(
        Platform.environment.containsKey('FLUTTER_TEST') ? p.join('hiddify-core', 'bin') : '',
        Platform.isWindows
            ? 'hiddify-core.dll'
            : Platform.isMacOS
            ? 'hiddify-core.dylib'
            : 'hiddify-core.so',
      ),
    ),
  );
  static const port = 17178;
  final _identity = RelayRpcIdentity();
  ClientChannel? _channel;

  @override
  Future<String> setup(Directories directories, bool debug, int mode) async {
    // Never attach to a foreign listener just because it answers Hello.
    final error = using(
      (arena) => _box
          .setup(
            directories.baseDir.path.toNativeUtf8(allocator: arena).cast(),
            directories.workingDir.path.toNativeUtf8(allocator: arena).cast(),
            directories.tempDir.path.toNativeUtf8(allocator: arena).cast(),
            SetupMode.GRPC_NORMAL.value,
            '127.0.0.1:$port'.toNativeUtf8(allocator: arena).cast(),
            ''.toNativeUtf8(allocator: arena).cast(),
            0,
            0,
          )
          .cast<Utf8>()
          .toDartString(),
    );
    if (error.isNotEmpty) return error;
    final serverPem = _box.GetServerPublicKey().cast<Utf8>().toDartString();
    final registrationError = using(
      (arena) => _box.AddGrpcClientPublicKey(
        utf8.decode(_identity.registration).toNativeUtf8(allocator: arena).cast(),
      ).cast<Utf8>().toDartString(),
    );
    if (registrationError.isNotEmpty) return registrationError;
    await _channel?.terminate();
    final channel = ClientChannel(
      '127.0.0.1',
      port: port,
      options: ChannelOptions(
        credentials: MTLSChannelCredentials(
          serverPublicKey: Uint8List.fromList(utf8.encode(serverPem)),
          identity: _identity,
        ),
      ),
    );
    _channel = channel;
    await HelloClient(channel).sayHello(
      HelloRequest(name: 'Relay Pilot'),
      options: CallOptions(timeout: const Duration(seconds: 5)),
    );
    fgClient = bgClient = CoreClient(channel);
    return '';
  }

  Future<void> dispose() async {
    await _channel?.terminate();
    _box.closeGrpc(SetupMode.GRPC_NORMAL.value);
  }
}
