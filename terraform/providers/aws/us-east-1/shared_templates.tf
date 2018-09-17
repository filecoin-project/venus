data "template_file" "docker_install" {
  template = "${file("../../../scripts/docker_install.sh")}"
}

data "template_file" "cadvisor_install" {
  template = "${file("../../../scripts/cadvisor_install.sh")}"
}
