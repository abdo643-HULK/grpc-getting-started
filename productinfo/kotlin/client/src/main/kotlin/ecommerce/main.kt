package ecommerce

import com.google.protobuf.stringValue
import io.grpc.*
import io.opencensus.common.Duration
import io.opencensus.contrib.grpc.metrics.RpcViews
import io.opencensus.exporter.stats.stackdriver.StackdriverStatsConfiguration
import io.opencensus.exporter.stats.stackdriver.StackdriverStatsExporter
import io.opencensus.trace.Tracing
import io.opencensus.trace.samplers.Samplers
import kotlinx.coroutines.TimeoutCancellationException
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import java.io.IOException
import java.nio.file.Paths
import java.util.logging.Logger
import javax.print.attribute.standard.Compression
import kotlin.time.Duration.Companion.seconds


const val PORT = 5001
const val HOST = "localhost"
const val TSL = true

fun main(): Unit = runBlocking {
//    // single tls connection
//    val certFile = Paths.get("secure-channel", "certs", "server.crt").toFile()
//    val tlsCredentials = TlsChannelCredentials.newBuilder().trustManager(certFile).build()

    val caFile = Paths.get("C:\\Code\\grpc-up-and-running\\", "productinfo", "certs", "ca.crt").toFile()
    val certFile = Paths.get("C:\\Code\\grpc-up-and-running\\", "productinfo", "certs", "client.crt").toFile()
    val keyFile = Paths.get("C:\\Code\\grpc-up-and-running\\", "productinfo", "certs", "client.pem").toFile()

    val tlsCredentials = TlsChannelCredentials
        .newBuilder()
        .trustManager(caFile)
        .keyManager(certFile, keyFile)
        .build()

    val channel = if (TSL) {
        Grpc.newChannelBuilderForAddress(HOST, PORT, tlsCredentials).build()
    } else {
        ManagedChannelBuilder
            .forAddress(HOST, PORT)
            .intercept(MetaDataInterceptor())
            .usePlaintext()
            .build()
    }

    setupOpencensusAndExporters()

    launch {
//        // =========== UNARY ===========
//        productTest(channel)
//
//        // =========== UNARY/STREAMING ===========
//        orderTest(channel)

        // =========== CREDENTIALS ===========
//        val callCredentials = BasicCallCredentials("admin", "admin")
        val callCredentials = TokenCallCredentials("some-secret-token")
        addWithCredentials(channel, callCredentials)

        channel.shutdown()
    }
}

@Throws(IOException::class)
private fun setupOpencensusAndExporters() {
    // For demo purposes, always sample
    val traceConfig = Tracing.getTraceConfig()
    traceConfig.updateActiveTraceParams(
        traceConfig.activeTraceParams
            .toBuilder()
            .setSampler(Samplers.alwaysSample())
            .build()
    )

    // Register all the gRPC views and enable stats
    RpcViews.registerAllGrpcViews()

    // Create the Stackdriver stats exporter
    StackdriverStatsExporter.createAndRegister(
        StackdriverStatsConfiguration.builder()
            .setProjectId("grpc-up-and-running")
            .setExportInterval(Duration.create(5, 0))
            .build()
    )
}

suspend fun addWithCredentials(channel: Channel, credentials: CallCredentials) {
    val stub = ProductInfoGrpcKt.ProductInfoCoroutineStub(channel).withCallCredentials(credentials)
    val tracer = Tracing.getTracer()

    val productID = tracer
        .spanBuilder("ecommerce.ProductInfoClient.addProduct")
        .startScopedSpan()
        .use { _ ->
            val productID = stub.addProduct(
                product {
                    name = "Apple iPhone 13"
                    description = "Meet Apple iPhone 13. " +
                            "All-mew triple-camera system with" +
                            " Ultra Wide and Night mode."
                    price = 100_000
                }
            )
            println("Product ID: ${productID.value} added successfully")
            productID
        }

    println("Product ${stub.getProduct(productID)}")
}

suspend fun orderTest(channel: Channel) {
    val logger = Logger.getLogger(object {}::class.java.`package`.name)
    val stub = OrderManagementGrpcKt.OrderManagementCoroutineStub(channel).withCompression(Compression.GZIP.toString())

    // =========== UNARY ===========
    val retrievedOrder = stub.getOrder(stringValue { value = "106" })
    logger.info("getOrder Response ->: $retrievedOrder")

    // =========== SERVER STREAMING ===========
    stub.searchOrders(stringValue { value = "Google" }).collect {
        logger.info("Search Result: $it")
    }

    try {
        withTimeout(10.seconds) {
            val updateRes = stub.updateOrders(flow {
                emit(order {
                    id = "102"
                    price = 110_000
                    destination = "Mountain View, CA"
                    items.addAll(arrayOf("Google Pixel 3A", "Google Pixel Book").asIterable())
                })
                emit(order {
                    id = "103"
                    price = 280_000
                    destination = "San Jose, CA"
                    items.addAll(arrayOf("Apple Watch S4", "Mac Book Pro", "iPad Pro").asIterable())
                })
                emit(order {
                    id = "104"
                    price = 220_000
                    destination = "Mountain View, CA"
                    items.addAll(arrayOf("Google Home Mini", "Google Nest Hub", "iPad Mini").asIterable())
                })
            })

            logger.info("Update Orders Res: $updateRes")
        }
    } catch (_e: StatusRuntimeException) {
        logger.severe("updateOrder got error: ${_e.message}")
        _e.printStackTrace()
    } catch (_e: TimeoutCancellationException) {
        logger.warning("FAILED: Update orders couldn't finish within 10 seconds");
    }

    // =========== BIDIRECTIONAL STREAM ===========
    try {
        withTimeout(10.seconds) {
            stub.processOrders(flow {
                emit(stringValue { value = "102" })
                emit(stringValue { value = "103" })
                emit(stringValue { value = "104" })
//                delay(2.seconds)
//                emit(stringValue { value = "101" })
            }).collect {
                logger.info("Combined Shipment : " + it.id + " : \n" + it.ordersListList)
            }
        }

        logger.info("Order Processing completed!")
    } catch (_e: StatusRuntimeException) {
        logger.severe("processOrders got error: ${_e.message}")
        _e.printStackTrace()
    } catch (_e: TimeoutCancellationException) {
        logger.warning("FAILED: Process orders couldn't finish within 10 seconds");
    }

    try {
        val result = stub.addOrder(order {
            id = "101" //"-1"
            price = 230_000
            destination = "San Jose, CA"
            items.addAll(arrayOf("iPhone XS", "Mac Book Pro").asIterable())
        })
        logger.info("AddOrder Response -> : " + result.value)
    } catch (_e: StatusRuntimeException) {
        logger.info("Error Received - Error Code : " + _e.status.code);
        logger.info("Error details -> " + _e.message);
    }
}

suspend fun productTest(channel: Channel) {
    val stub = ProductInfoGrpcKt.ProductInfoCoroutineStub(channel).withCompression(Compression.GZIP.toString())

    val productID = stub.addProduct(
        product {
            name = "Apple iPhone 13"
            description = "Meet Apple iPhone 13. " +
                    "All-mew triple-camera system with" +
                    " Ultra Wide and Night mode."
            price = 100_000
        }
    )
    println("Product ID: ${productID.value} added successfully")
    println("Product ${stub.getProduct(productID)}")
}