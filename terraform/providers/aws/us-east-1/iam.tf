resource "aws_iam_instance_profile" "filecoin_kittyhawk" {
  name = "filecoin_kittyhawk"
  role = "${aws_iam_role.filecoin_kittyhawk.name}"
}

resource "aws_iam_role" "filecoin_kittyhawk" {
  name = "filecoin_kittyhawk"
  path = "/"

  assume_role_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "sts:AssumeRole",
            "Principal": {
               "Service": "ec2.amazonaws.com"
            },
            "Effect": "Allow",
            "Sid": ""
        }
    ]
}
EOF
}

resource "aws_iam_policy" "ecr_allow_all" {
  name        = "ecr_allow_all"
  description = "Allow ECR access"
  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "ecr:*"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "filecoin_kittyhawk-ecr_allow_all" {
  role       = "${aws_iam_role.filecoin_kittyhawk.name}"
  policy_arn = "${aws_iam_policy.ecr_allow_all.arn}"
}
