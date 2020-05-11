matrixJob('Periodic go-systemd builder') {
    label('master')
    displayName('Periodic go-systemd builder (master branch)')

    scm {
        git {
            remote {
                url('https://github.com/coreos/go-systemd.git')
            }
            branch('master')
        }
    }

    concurrentBuild()

    triggers {
        cron('@daily')
    }

    axes {
        label('os_type', 'debian-testing', 'fedora-24', 'fedora-25')
    }

    wrappers {
        buildNameSetter {
            template('go-systemd master (periodic #${BUILD_NUMBER})')
            runAtStart(true)
            runAtEnd(true)
        }
        timeout {
            absolute(25)
        }
    }

    steps {
        shell('./scripts/jenkins/periodic-go-systemd-builder.sh')
    }
}