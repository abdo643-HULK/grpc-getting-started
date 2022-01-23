plugins {
    idea
    application
    kotlin("jvm")
}

repositories {
    mavenCentral()
}

dependencies {
    implementation(project(":stub"))

    runtimeOnly("io.grpc:grpc-netty:${Constants.grpcVersion}")
}

tasks.named("startScripts") {
    enabled = false
}