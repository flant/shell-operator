@Library('jenkins.shared.library') _

pipeline {
  agent {
    label 'ubuntu_docker_label'
  }
  environment {
    REGISTRY = "core-harbor-prod.sdp.infoblox.com"
    VERSION = sh(script: "git describe --always --long --tags", returnStdout: true).trim()
    TAG = "${env.VERSION}-j${env.BUILD_NUMBER}"
  }
  stages {
    stage("Build Image") {
      steps {
        sh 'docker build . -t tags-operator:$TAG'
      }
    }
    stage("Push Image") {
      when {
        anyOf {
          branch 'master'
          branch 'jenkinsfile'
        }
      }
      steps {
        script {
          signDockerImage('tags-operator', env.TAG, 'infoblox')
        }
      }
    }
  }
  post {
    success {
       finalizeBuild()
    }
  }
}
