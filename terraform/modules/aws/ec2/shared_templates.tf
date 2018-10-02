data "template_file" "docker_install" {
  template = "${file("../../../scripts/docker_install.sh")}"
}

data "template_file" "cadvisor_install" {
  template = "${file("../../../scripts/cadvisor_install.sh")}"
}

data "template_file" "node_exporter_install" {
  template = "${file("../../../scripts/node_exporter_install.sh")}"
}

data "template_file" "setup_instance_storage" {
  template = "${file("../../../scripts/setup_instance_storage.sh")}"
}
