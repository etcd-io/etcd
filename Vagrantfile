# This Vagrantfile is targeted at developers. It can be used to build and run etcd in an isolated VM.

$provision = <<SCRIPT

apt-get update
apt-get install -y python-software-properties git
add-apt-repository -y ppa:duh/golang
apt-get update
apt-get install -y golang

cd /vagrant && ./build

/vagrant/etcd -c 0.0.0.0:4001 -s 0.0.0.0:7001 &

SCRIPT


Vagrant.configure("2") do |config|
    config.vm.box = 'precise64'
    config.vm.box_url = 'http://files.vagrantup.com/precise64.box'

    config.vm.provider "virtualbox" do |vbox|
        vbox.customize ["modifyvm", :id, "--memory", "1024"]
    end

    config.vm.provision "shell", inline: $provision

    config.vm.network "forwarded_port", guest: 4001, host: 4001, auto_correct: true
    config.vm.network "forwarded_port", guest: 7001, host: 7001, auto_correct: true

end
