import com.google.protobuf.gradle.*
import org.jetbrains.kotlin.gradle.tasks.KotlinCompile

plugins {
    `java-library`
    kotlin("jvm")
    id("com.google.protobuf")
}

repositories {
    mavenCentral()
}

sourceSets {
    main {
        proto {
            srcDir("../../../proto")
        }
    }
}

dependencies {
    api(kotlin("stdlib"))

    api("org.jetbrains.kotlinx:kotlinx-coroutines-core:${Constants.coroutinesVersion}")

    api("io.grpc:grpc-stub:${Constants.grpcVersion}")
    api("io.grpc:grpc-protobuf:${Constants.grpcVersion}")
    api("io.grpc:grpc-kotlin-stub:${Constants.grpcKotlinVersion}")

    api("com.google.protobuf:protobuf-kotlin:${Constants.protobufVersion}")
    api("com.google.protobuf:protobuf-java-util:${Constants.protobufVersion}")

    api("io.opencensus:opencensus-api:${Constants.opencensusVersion}")
    api("io.opencensus:opencensus-impl:${Constants.opencensusVersion}")
    api("io.opencensus:opencensus-contrib-grpc-metrics:${Constants.opencensusVersion}")
    api("io.opencensus:opencensus-exporter-trace-stackdriver:${Constants.opencensusVersion}")
    api("io.opencensus:opencensus-exporter-stats-stackdriver:${Constants.opencensusVersion}")
}

buildscript {
    repositories {
        mavenCentral()
    }

    dependencies {
        classpath("com.google.protobuf:protobuf-gradle-plugin:0.8.17")
        classpath("org.jetbrains.kotlin:kotlin-gradle-plugin:${Constants.grpcKotlinVersion}")
    }
}

protobuf {
    protoc {
        artifact = "com.google.protobuf:protoc:${Constants.protobufVersion}"
    }

    plugins {
        id("grpc") {
            artifact = "io.grpc:protoc-gen-grpc-java:${Constants.grpcVersion}"
        }
        id("grpckt") {
            artifact = "io.grpc:protoc-gen-grpc-kotlin:${Constants.grpcKotlinVersion}:jdk7@jar"
        }
    }

    generateProtoTasks {
        ofSourceSet("main").forEach {
            it.plugins {
                id("grpc")
                id("grpckt")
            }
            it.builtins {
                id("kotlin")
            }
        }
    }
}

tasks.withType<KotlinCompile>().all {
    kotlinOptions {
        freeCompilerArgs = listOf("-Xopt-in=kotlin.RequiresOptIn")
    }
}