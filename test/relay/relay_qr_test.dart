import 'dart:io';
import 'dart:typed_data';
import 'package:flutter_test/flutter_test.dart';
import 'package:hiddify/features/relay/relay_qr.dart';

void main() {
  test('QR PNG imports the exact personal link offline', () {
    expect(
      decodeRelayQr(File('test/relay/access-demo.png').readAsBytesSync()),
      'https://pilot.example/sub/${'A' * 43}',
    );
  });
  test('invalid image is rejected', () {
    expect(() => decodeRelayQr(Uint8List.fromList([1, 2, 3])), throwsFormatException);
  });
}
