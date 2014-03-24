# -*- mode: ruby -*-
# vi: set ft=ruby :
#
Vagrant.require_version '>= 1.5.0'
Vagrant.configure("2") do |config|
  config.vm.box = "precise64"
  config.vm.box_url = "http://files.vagrantup.com/precise64.box"

  config.vm.network :forwarded_port, host: 4001, guest: 4001
  config.vm.network :forwarded_port, host: 7001, guest: 7001

  # Fix docker not being able to resolve private registry in VirtualBox
  config.vm.provider :virtualbox do |vb, override|
    vb.customize ["modifyvm", :id, "--natdnshostresolver1", "on"]
    vb.customize ["modifyvm", :id, "--natdnsproxy1", "on"]
  end

  config.vm.provision "docker" do |d|
    d.build_image "/vagrant", args: '-t etcd'
    d.run "etcd", args: "-p 4001:4001 -p 7001:7001", demonize: true
  end

  # plugin conflict
  if Vagrant.has_plugin?("vagrant-vbguest")
    config.vbguest.auto_update = false
  end
end
