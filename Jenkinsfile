pipeline {
    options {
        timeout(time: 1, unit: 'HOURS')
    }
    agent {
        docker {
            image 'teambitflow/golang-build:1.12-stretch'
            args '-v /root/.goroot:/go -v /var/run/docker.sock:/var/run/docker.sock'
        }
    }
    environment {
        registry = 'teambitflow/bitflow-collector'
        registryCredential = 'dockerhub'
        dockerImage = '' // Empty variable must be declared here to allow passing an object between the stages.
        dockerImageARM32 = ''
        dockerImageARM64 = ''
    }
    stages {
        stage('Git') {
            steps {
                script {
                    env.GIT_COMMITTER_EMAIL = sh(
                        script: "git --no-pager show -s --format='%ae'",
                        returnStdout: true
                        ).trim()
                }
            }
        }
        stage('Build & test') { 
            steps {
                    sh 'apt update'
                    sh 'apt install -y libvirt-dev libpcap-dev'
                    sh 'go clean -i -v ./...'
                    sh 'go install -v ./...'
                    sh 'rm -rf reports && mkdir -p reports'
                    sh 'go test -v ./... -coverprofile=reports/test-coverage.txt 2>&1 | go-junit-report > reports/test.xml'
                    sh 'go vet ./... &> reports/vet.txt'
                    sh 'golint $(go list -f "{{.Dir}}" ./...) &> reports/lint.txt'
            }
            post {
                always {
                    archiveArtifacts 'reports/*'
                    junit 'reports/test.xml'
                }
            }
        }
        stage('SonarQube') {
            steps {
                script {
                    // sonar-scanner which don't rely on JVM
                    def scannerHome = tool 'sonar-scanner-linux'
                    withSonarQubeEnv('CIT SonarQube') {
                        sh """
                            ${scannerHome}/bin/sonar-scanner -Dsonar.projectKey=go-bitflow-collector -Dsonar.branch.name=$BRANCH_NAME \
                                -Dsonar.sources=. -Dsonar.tests=. \
                                -Dsonar.inclusions="**/*.go" -Dsonar.test.inclusions="**/*_test.go" \
                                -Dsonar.go.golint.reportPath=reports/lint.txt \
                                -Dsonar.go.govet.reportPaths=reports/vet.txt \
                                -Dsonar.go.coverage.reportPaths=reports/test-coverage.txt \
                                -Dsonar.test.reportPath=reports/test.xml
                        """
                    }
                }
                timeout(time: 10, unit: 'MINUTES') {
                    waitForQualityGate abortPipeline: true
                }
            }
        }
        stage('Docker build') {
            steps {
                script {
                    dockerImage = docker.build registry + ':$BRANCH_NAME-build-$BUILD_NUMBER'
                    dockerImageARM32 = docker.build registry + ':$BRANCH_NAME-build-$BUILD_NUMBER-arm32v7', '-f arm32v7.Dockerfile .'
                    dockerImageARM64 = docker.build registry + ':$BRANCH_NAME-build-$BUILD_NUMBER-arm64v8', '-f arm64v8.Dockerfile .'
                }
            }
        }
        stage('Docker push') {
            when {
                branch 'master'
            }
            steps {
                script {
                    docker.withRegistry('', registryCredential) {
                        dockerImage.push("build-$BUILD_NUMBER")
                        dockerImage.push("latest-amd64")
                        dockerImageARM32.push("build-$BUILD_NUMBER-arm32v7")
                        dockerImageARM32.push("latest-arm32v7")
                        dockerImageARM64.push("build-$BUILD_NUMBER-arm64v8")
                        dockerImageARM64.push("latest-arm64v8")
                    }
                }
                withCredentials([
                  [
                    $class: 'UsernamePasswordMultiBinding',
                    credentialsId: 'dockerhub',
                    usernameVariable: 'DOCKERUSER',
                    passwordVariable: 'DOCKERPASS'
                  ]
                ]) {
                    // Dockerhub Login
                    sh '''#! /bin/bash
                    echo $DOCKERPASS | docker login -u $DOCKERUSER --password-stdin
                    '''
                    // teambitflow/bitflow4j:latest manifest
                    sh "docker manifest create ${registry}:latest ${registry}:latest-amd64 ${registry}:latest-arm32v7 ${registry}:latest-arm64v8"
                    sh "docker manifest annotate ${registry}:latest ${registry}:latest-arm32v7 --os=linux --arch=arm --variant=v7"
                    sh "docker manifest annotate ${registry}:latest ${registry}:latest-arm64v8 --os=linux --arch=arm64 --variant=v8"
                    sh "docker manifest push --purge ${registry}:latest"
                }
            }
        }
    }
    post {
        success {
            withSonarQubeEnv('CIT SonarQube') {
                slackSend channel: '#jenkins-builds-all', color: 'good',
                    message: "Build ${env.JOB_NAME} ${env.BUILD_NUMBER} was successful (<${env.BUILD_URL}|Open Jenkins>) (<${env.SONAR_HOST_URL}|Open SonarQube>)"
            }
        }
        failure {
            slackSend channel: '#jenkins-builds-all', color: 'danger',
                message: "Build ${env.JOB_NAME} ${env.BUILD_NUMBER} failed (<${env.BUILD_URL}|Open Jenkins>)"
        }
        fixed {
            withSonarQubeEnv('CIT SonarQube') {
                slackSend channel: '#jenkins-builds', color: 'good',
                    message: "Thanks to ${env.GIT_COMMITTER_EMAIL}, build ${env.JOB_NAME} ${env.BUILD_NUMBER} was successful (<${env.BUILD_URL}|Open Jenkins>) (<${env.SONAR_HOST_URL}|Open SonarQube>)"
            }
        }
        regression {
            slackSend channel: '#jenkins-builds', color: 'danger',
                message: "What have you done ${env.GIT_COMMITTER_EMAIL}? Build ${env.JOB_NAME} ${env.BUILD_NUMBER} failed (<${env.BUILD_URL}|Open Jenkins>)"
        }
    }
}

