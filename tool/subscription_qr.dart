import 'dart:io';
import 'package:image/image.dart' as img;
import 'package:zxing2/qrcode.dart';
import 'package:hiddify/features/relay/relay_profile.dart';

// Read the secret from stdin, not an argument recorded in process lists.
void main(List<String> args) {
  if (args.length != 1) throw ArgumentError('dart run tool/subscription_qr.dart OUTPUT.png < private-url.txt');
  final link = RelayProfile.subscriptionUri(stdin.readLineSync() ?? '').toString();
  final matrix = Encoder.encode(link, ErrorCorrectionLevel.m).matrix!;
  const scale = 8, border = 4;
  final image = img.Image(
    width: (matrix.width + border * 2) * scale,
    height: (matrix.height + border * 2) * scale,
    numChannels: 3,
  );
  img.fill(image, color: img.ColorRgb8(255, 255, 255));
  for (var y = 0; y < matrix.height; y++) {
    for (var x = 0; x < matrix.width; x++) {
      if (matrix.get(x, y) == 1) {
        img.fillRect(
          image,
          x1: (x + border) * scale,
          y1: (y + border) * scale,
          x2: (x + border + 1) * scale - 1,
          y2: (y + border + 1) * scale - 1,
          color: img.ColorRgb8(0, 0, 0),
        );
      }
    }
  }
  File(args.single).writeAsBytesSync(img.encodePng(image));
}
