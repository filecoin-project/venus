resource "aws_route53_zone" "kittyhawk" {
  name = "kittyhawk.wtf"
}

resource "aws_route53_record" "service" {
  name    = "filecoin.${aws_route53_zone.kittyhawk.name}"
  zone_id = "${aws_route53_zone.kittyhawk.zone_id}"
  type    = "A"
  records = ["${aws_eip.filecoin.public_ip}"]
  ttl     = "30"
}
