pipeline {
    options {
        timeout(time: 1, unit: 'HOURS')
    }
    agent none
    environment {
        registry = 'teambitflow/bitflow-collector'
        registryCredential = 'dockerhub'
        dockerImage = '' // Empty variable must be declared here to allow passing an object between the stages.
        dockerImageARM32 = ''
        dockerImageARM64 = ''
    }
    stages {
        stage('Build & test') {
            agent {
                docker {
                    image 'teambitflow/bitflow-collector-build:debian'
                    args '-v /tmp/go-mod-cache/debian:/go'
                }
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
            }
        }
        stage('Docker alpine') {
            agent {
                docker {
                    image 'teambitflow/bitflow-collector-build:alpine'
                    args '-v /tmp/go-mod-cache/alpine:/go -v /var/run/docker.sock:/var/run/docker.sock'
                }
            }
            stages {
                stage('Docker build') {
                    steps {
                        sh './build/native-build.sh'
                        script {
                            dockerImage = docker.build registry + ':$BRANCH_NAME-build-$BUILD_NUMBER', '-f build/alpine-prebuilt.Dockerfile build/_output/native'
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
                            }
                        }
                    }
                }
            }
        }
/*
        stage('Docker arm32v7') {
            agent {
                docker {
                    image 'teambitflow/bitflow-collector-build:arm32v7'
                    args '-v /tmp/go-mod-cache/arm32v7:/go -v /var/run/docker.sock:/var/run/docker.sock'
                }
            }
            stages {
                stage('Docker build') {
                    steps {
                        sh './build/native-build.sh -tags nolibvirt'
                        script {
                            dockerImageARM32 = docker.build registry + ':$BRANCH_NAME-build-$BUILD_NUMBER-arm32v7', '-f build/arm32v7-prebuilt.Dockerfile build/_output/native'
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
                                dockerImageARM32.push("build-$BUILD_NUMBER-arm32v7")
                                dockerImageARM32.push("latest-arm32v7")
                            }
                        }
                    }
                }
            }
        }
        stage('Docker arm64v8') {
            agent {
                docker {
                    image 'teambitflow/bitflow-collector-build:arm64v8'
                    args '-v /tmp/go-mod-cache/arm64v8:/go -v /var/run/docker.sock:/var/run/docker.sock'
                }
            }
            stages {
                stage('Docker build') {
                    steps {
                        sh './build/native-build.sh -tags nolibvirt'
                        script {
                            dockerImageARM64 = docker.build registry + ':$BRANCH_NAME-build-$BUILD_NUMBER-arm64v8', '-f build/arm64v8-prebuilt.Dockerfile build/_output/native'
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
                                dockerImageARM64.push("build-$BUILD_NUMBER-arm64v8")
                                dockerImageARM64.push("latest-arm64v8")
                            }
                        }
                    }
                }
            }
        }
        stage('Docker manifests') {
            when {
               branch 'master'
            }
            agent {
                docker {
                    image 'teambitflow/bitflow-collector-build:debian'
                    args '-v /var/run/docker.sock:/var/run/docker.sock'
                }
            }
            steps {
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
*/
    }
    post {
        success {
            node('master') {
                withSonarQubeEnv('CIT SonarQube') {
                    slackSend channel: '#jenkins-builds-all', color: 'good',
                        message: "Build ${env.JOB_NAME} ${env.BUILD_NUMBER} was successful (<${env.BUILD_URL}|Open Jenkins>) (<${env.SONAR_HOST_URL}|Open SonarQube>)"
                }
            }
        }
        failure {
            node('master') {
                slackSend channel: '#jenkins-builds-all', color: 'danger',
                    message: "Build ${env.JOB_NAME} ${env.BUILD_NUMBER} failed (<${env.BUILD_URL}|Open Jenkins>)"
            }
        }
        fixed {
            node('master') {
                withSonarQubeEnv('CIT SonarQube') {
                    slackSend channel: '#jenkins-builds', color: 'good',
                        message: "Thanks to ${env.GIT_COMMITTER_EMAIL}, build ${env.JOB_NAME} ${env.BUILD_NUMBER} was successful (<${env.BUILD_URL}|Open Jenkins>) (<${env.SONAR_HOST_URL}|Open SonarQube>)"
                }
            }
        }
        regression {
            node('master') {
                slackSend channel: '#jenkins-builds', color: 'danger',
                    message: "What have you done ${env.GIT_COMMITTER_EMAIL}? Build ${env.JOB_NAME} ${env.BUILD_NUMBER} failed (<${env.BUILD_URL}|Open Jenkins>)"
            }
        }
    }
}
