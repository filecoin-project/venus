data "template_file" "docker_install" {
  template = "${file("../../../scripts/docker_install.sh")}"
}

