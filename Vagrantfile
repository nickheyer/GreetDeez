# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
  config.ssh.pty = true

  config.vm.network :public_network, dev: "br0", mode: "bridge", type: "bridge"

  config.vm.provider :libvirt do |lv|
    lv.memory = 4096
    lv.cpus = 2
    lv.graphics_type = "spice"
    lv.video_type = "qxl"
    lv.management_network_mode = "none" 
  end

  create_test_user = <<~SHELL
    id test &>/dev/null || useradd -m -s /bin/bash test
    echo 'test:test' | chpasswd
    echo 'test ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/test
  SHELL

  # --- Arch Linux ---
  config.vm.define "arch", autostart: false do |arch|
    arch.vm.box = "generic/arch"
    arch.vm.hostname = "greetdeez-arch"

    arch.vm.provision "shell", inline: <<~SHELL
      echo 'nameserver 1.1.1.1' > /etc/resolv.conf
      pacman -Sy --noconfirm archlinux-keyring
      pacman -Syu --noconfirm
      pacman -S --noconfirm --needed sway gnome-session xorg-server plasma-desktop base-devel git

      #{create_test_user}
      su - test -c '
        git clone https://aur.archlinux.org/yay.git /tmp/yay
        cd /tmp/yay
        makepkg -si --noconfirm
      '

      su - test -c 'yay -S --noconfirm greetdeez-bin'
    SHELL
  end

  # --- Debian ---
  config.vm.define "debian", autostart: false do |deb|
    deb.vm.box = "debian/bookworm64"
    deb.vm.hostname = "greetdeez-debian"

    deb.vm.provision "shell", inline: <<~SHELL
      export DEBIAN_FRONTEND=noninteractive
      apt-get update
      apt-get install -y curl sway gnome-session plasma-desktop xorg

      curl -1sLf 'https://dl.cloudsmith.io/public/nickheyer/greetdeez/setup.deb.sh' | bash
      apt-get install -y greetdeez

      #{create_test_user}
    SHELL
  end

  # --- Fedora ---
  config.vm.define "fedora", autostart: false do |fed|
    fed.vm.box = "fedora/41-cloud-base"
    fed.vm.hostname = "greetdeez-fedora"

    fed.vm.provision "shell", inline: <<~SHELL
      dnf install -y sway gnome-session plasma-desktop @base-x
      curl -1sLf 'https://dl.cloudsmith.io/public/nickheyer/greetdeez/setup.rpm.sh' | bash
      dnf install -y greetdeez

      #{create_test_user}
    SHELL
  end

  # --- Alpine ---
  config.vm.define "alpine", autostart: false do |alp|
    alp.vm.box = "generic/alpine319"
    alp.vm.hostname = "greetdeez-alpine"

    alp.vm.provision "shell", inline: <<~SHELL
      apk update
      apk add sway gnome-session plasma-desktop xorg-server

      curl -1sLf 'https://dl.cloudsmith.io/public/nickheyer/greetdeez/setup.alpine.sh' | bash
      apk add greetdeez

      #{create_test_user}
    SHELL
  end
end
