import 'dart:typed_data';
import 'package:image/image.dart' as img;
import 'package:zxing2/qrcode.dart';
import 'package:hiddify/features/relay/relay_profile.dart';

String decodeRelayQr(Uint8List bytes) {
  if (bytes.length < 32) throw const FormatException('Изображение повреждено.');
  if (bytes.length > 8 * 1024 * 1024) throw const FormatException('Файл слишком большой.');
  final decoder = img.findDecoderForData(bytes);
  final info = decoder?.startDecode(bytes);
  if (info == null || info.width * info.height > 16000000) throw const FormatException('Неподдерживаемое изображение.');
  final picture = decoder!.decodeFrame(0);
  if (picture == null) throw const FormatException('Изображение повреждено.');
  final source = RGBLuminanceSource(
    picture.width,
    picture.height,
    picture.convert(numChannels: 4).getBytes(order: img.ChannelOrder.abgr).buffer.asInt32List(),
  );
  final value = QRCodeReader().decode(BinaryBitmap(HybridBinarizer(source))).text;
  return RelayProfile.subscriptionUri(value).toString();
}
