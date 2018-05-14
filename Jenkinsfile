#!groovy

properties([
    buildDiscarder(logRotator(daysToKeepStr: '30', numToKeepStr: '5')),

    [$class: 'CopyArtifactPermissionProperty',
     projectNames: '*'],

    parameters([
        booleanParam(name: 'HAVE_ARM64_NODE',
            defaultValue: true),
    ]),

    pipelineTriggers([pollSCM('H/15 * * * *')])
])

def buildMantle = { String _arch ->
    String arch = _arch

    node("${arch} && docker") {
        stage("${arch}: SCM") {
            checkout scm
        }
        stage("${arch}: Builder") {
            sh "/bin/bash -x ./docker/build-builder --rebuild"
        }
        stage("${arch}: Mantle") {
            sh "/bin/bash -x ./docker/build-mantle"
        }
        stage("${arch}: Test") {
            sh "/bin/bash -x ./docker/build-mantle --test"
        }
        stage("${arch}: Runner") {
            sh "/bin/bash -x ./docker/build-runner --rebuild"
        }
        stage("${arch}: Post-build") {
            if (env.JOB_BASE_NAME == "master-builder") {
                sh "docker save \$(./docker/build-runner --tag) > mantle-runner-${arch}.tar && \
                    cp -v run-mantle run-mantle-${arch}"
                archiveArtifacts(artifacts: "mantle-runner-${arch}.tar, run-mantle-${arch}",
                    fingerprint: true, onlyIfSuccessful: true)

                // Legacy artifacts.
                if (arch == "amd64") {
                    archiveArtifacts(artifacts: 'bin/**', fingerprint: true,
                        onlyIfSuccessful: true)
                }
                sh "cp -av \$(find bin -maxdepth 1 -type f) bin/${arch}/"
                archiveArtifacts(artifacts: "bin/${arch}/*", fingerprint: true,
                    onlyIfSuccessful: true)
            }
        }
    }
}

def build_map = [:]

build_map.failFast = false
build_map['amd64'] = buildMantle.curry('amd64')
if (params.HAVE_ARM64_NODE) {
    build_map['arm64'] = buildMantle.curry('arm64')
}

parallel build_map
