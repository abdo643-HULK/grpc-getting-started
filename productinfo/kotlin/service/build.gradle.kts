import com.google.protobuf.gradle.*
import org.jetbrains.kotlin.gradle.tasks.KotlinCompile

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
    implementation("io.grpc:grpc-services:${Constants.grpcVersion}")

    runtimeOnly("io.grpc:grpc-netty:${Constants.grpcVersion}")
}

tasks.jar {
    manifest {
        attributes("Main-Class" to "MainKt")
    }
}

tasks.named("startScripts") {
    enabled = false
}

//sourceSets {
//    main {
//        proto {
//            srcDir("../../../proto")
//        }
//    }
//}

//buildscript {
//    repositories {
//        mavenCentral()
//    }
//
//    dependencies {
//        classpath("com.google.protobuf:protobuf-gradle-plugin:0.8.17")
//        classpath("org.jetbrains.kotlin:kotlin-gradle-plugin:${Constants.grpcKotlinVersion}")
//    }
//}

//
//buildscript {
//    repositories {
//        mavenCentral()
//    }
//
//    dependencies {
//        classpath("com.google.protobuf:protobuf-gradle-plugin:0.8.17")
//        classpath("org.jetbrains.kotlin:kotlin-gradle-plugin:${Constants.grpcKotlinVersion}")
//    }
//}
//
//protobuf {
//    protoc {
//        artifact = "com.google.protobuf:protoc:${Constants.protobufVersion}"
//    }
//
//    plugins {
//        id("grpc") {
//            artifact = "io.grpc:protoc-gen-grpc-java:${Constants.grpcVersion}"
//        }
//        id("grpckt") {
//            artifact = "io.grpc:protoc-gen-grpc-kotlin:${Constants.grpcKotlinVersion}:jdk7@jar"
//        }
//    }
//
//    generateProtoTasks {
//        ofSourceSet("main").forEach {
//            it.plugins {
//                id("grpc")
//                id("grpckt")
//            }
//            it.builtins {
//                id("kotlin")
//            }
//        }
//    }
//}

//task productInfoServer(type: CreateStartScripts) {
//    mainClassName = 'ecommerce.ProductInfoServerKt'
//    applicationName = 'productinfo-server'
//    outputDir = startScripts.outputDir
//    classpath = startScripts.classpath
//}


//tasks.withType<KotlinCompile>().all {
//    kotlinOptions {
//        freeCompilerArgs = listOf("-Xopt-in=kotlin.RequiresOptIn")
//    }
//}