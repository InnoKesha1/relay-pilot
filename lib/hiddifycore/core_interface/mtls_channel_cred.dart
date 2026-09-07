import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'package:basic_utils/basic_utils.dart';
import 'package:grpc/grpc.dart';

/// Ephemeral identity: the client private key never leaves this process.
class RelayRpcIdentity {
  final AsymmetricKeyPair<PublicKey, PrivateKey> key = CryptoUtils.generateEcKeyPair();
  late final String certificate = X509Utils.generateSelfSignedCertificate(
    key.privateKey,
    X509Utils.generateEccCsrPem(
      {'CN': 'Relay Pilot client'},
      key.privateKey as ECPrivateKey,
      key.publicKey as ECPublicKey,
    ),
    365,
    notBefore: DateTime.now().subtract(const Duration(minutes: 1)),
  );
  // Core 4.1.0 requires this PEM label even for a complete certificate.
  Uint8List get registration => Uint8List.fromList(utf8.encode(certificate.replaceAll('CERTIFICATE', 'PUBLIC KEY')));
}

class MTLSChannelCredentials extends ChannelCredentials {
  final SecurityContext ctx = SecurityContext(withTrustedRoots: false);
  MTLSChannelCredentials({required Uint8List serverPublicKey, required RelayRpcIdentity identity})
    : super.secure(
        authority: '127.0.0.1',
        onBadCertificate: (certificate, host) {
          // Upstream has no localhost SAN. Pin the exact certificate obtained
          // through FFI / Android method channel, including its validity period.
          final now = DateTime.now();
          return host == '127.0.0.1' &&
              now.isAfter(certificate.startValidity) &&
              now.isBefore(certificate.endValidity) &&
              _body(certificate.pem) == _body(utf8.decode(serverPublicKey));
        },
      ) {
    ctx.setAlpnProtocols(['h2'], false);
    ctx.setTrustedCertificatesBytes(serverPublicKey);
    ctx.usePrivateKeyBytes(utf8.encode(CryptoUtils.encodeEcPrivateKeyToPem(identity.key.privateKey as ECPrivateKey)));
    ctx.useCertificateChainBytes(utf8.encode(identity.certificate));
  }
  static String _body(String pem) => pem.replaceAll(RegExp(r'-----[^-]+-----|\s'), '');
  @override
  SecurityContext get securityContext => ctx;
}
