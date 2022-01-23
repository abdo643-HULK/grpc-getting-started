package ecommerce

import io.grpc.Grpc
import io.grpc.ServerBuilder
import io.grpc.ServerInterceptors
import io.grpc.TlsServerCredentials
import io.grpc.TlsServerCredentials.ClientAuth
import io.grpc.protobuf.services.ProtoReflectionService
import io.opencensus.common.Duration
import io.opencensus.contrib.grpc.metrics.RpcViews
import io.opencensus.exporter.stats.stackdriver.StackdriverStatsConfiguration
import io.opencensus.exporter.stats.stackdriver.StackdriverStatsExporter
import io.opencensus.trace.Tracing
import io.opencensus.trace.samplers.Samplers
import java.nio.file.Paths


const val PORT = 5001
const val TLS = true

fun main(_ignore: Array<String>) {
    val certFile = Paths.get("C:\\Code\\grpc-up-and-running\\", "productinfo", "certs", "server.crt").toFile()
    val keyFile = Paths.get("C:\\Code\\grpc-up-and-running\\", "productinfo", "certs", "server.pem").toFile()
    val caFile = Paths.get("C:\\Code\\grpc-up-and-running\\", "productinfo", "certs", "ca.crt").toFile()

    val serverBuilder = if (TLS) {
        val tlsCredentials = TlsServerCredentials
            .newBuilder()
            .keyManager(certFile, keyFile)
            .trustManager(caFile)
            .clientAuth(ClientAuth.OPTIONAL)
            .build()

        Grpc.newServerBuilderForPort(PORT, tlsCredentials)
        //.useTransportSecurity(certFile, keyFile)
    } else {
        ServerBuilder.forPort(PORT)
    }

    // For demo purposes, always sample
    val traceConfig = Tracing.getTraceConfig()
    traceConfig.updateActiveTraceParams(
        traceConfig.activeTraceParams
            .toBuilder()
            .setSampler(Samplers.alwaysSample())
            .build()
    )

    RpcViews.registerAllGrpcViews()
    // Create the Stackdriver trace exporter
    StackdriverStatsExporter.createAndRegister(
        StackdriverStatsConfiguration.builder()
            .setProjectId("grpc-up-and-running")
            .setExportInterval(Duration.create(5, 0))
            .build()
    )

    val server = serverBuilder
        // this interceptor is for all Services
        .intercept(AuthInterceptor())
        .intercept(MetaDataInterceptor())
        .addService(ProtoReflectionService.newInstance())
        .addService(ProductInfoService())
        // Java Api uses one Interface for streaming and unary interceptors
        .addService(
            ServerInterceptors
                .intercept(OrderManagementService(), OrderServerInterceptor())
        )
        .build()

    server.start()
    println("Server started, listening on $PORT")
    Runtime.getRuntime().addShutdownHook(
        Thread {
            println("Shutting down gRPC server since JVM is shutting down")
            server?.shutdown()
            println("Server shut down")
        }
    )

    server.awaitTermination()
}