package ecommerce

import com.google.protobuf.StringValue
import com.google.protobuf.stringValue
import io.grpc.Status
import kotlinx.coroutines.flow.*
import java.util.logging.Logger
import kotlin.random.Random


class OrderManagementService : OrderManagementGrpcKt.OrderManagementCoroutineImplBase() {
    companion object {
        private const val BATCH_SIZE = 3
        private val logger = Logger.getLogger(OrderManagementService::class.java.name)
    }

    private val orderMap = mutableMapOf<String, Order>()
    private val combinedShipmentMap = mutableMapOf<String, CombinedShipment>()

    init {
        initData()
    }

    // Unary
    override suspend fun addOrder(request: Order): StringValue {
        if (request.id == "-1") {
            println("Order ID is invalid! -> Received Order ID ${request.id}")
            throw Status
                .INVALID_ARGUMENT
                .withDescription("Invalid order ID received.")
                .asException()
        }

        orderMap[request.id] = request
        return stringValue { value = "100500" }
    }

    // Unary
    override suspend fun getOrder(request: StringValue): Order {
        val order = orderMap[request.value]
            ?: throw Status
                .NOT_FOUND
                .withDescription("Order doesn't exist")
                .asException()

        println("Order Retrieved: ID - ${order.id}")
        return order
    }

    // Server Streaming
    override fun searchOrders(request: StringValue): Flow<Order> = flow {
        for (orderEntry in orderMap) {
            val order = orderEntry.value
            val itemsCount = order.itemsCount
            for (i in 0 until itemsCount) {
                val item = order.getItems(i)
                if (item.contains(request.value)) {
                    logger.info("Item found $item")
                    emit(order)
                }
            }
        }
    }

    // Client Streaming
    override suspend fun updateOrders(requests: Flow<Order>): StringValue {
        val updatedOrderStr = StringBuilder().append("Updated Order IDs: ")

        requests.collect {
            orderMap[it.id] = it
            updatedOrderStr.append(it.id).append(", ")
            logger.info("Order ID: ${it.id} - Updated")
        }

        logger.info("Update orders - Completed")
        return stringValue { value = updatedOrderStr.toString() }
    }

    override fun processOrders(requests: Flow<StringValue>): Flow<CombinedShipment> = flow {
        requests.collectIndexed { i, orderId ->
            logger.info("Order Proc : ID - " + orderId.value)

            val currentOrder = orderMap[orderId.value]
                ?: throw Status
                    .NOT_FOUND
                    .withDescription("No order found. ID - ${orderId.value}")
                    .asException()

            val destination = currentOrder.destination
            val existingShipment = combinedShipmentMap[destination]

            combinedShipmentMap[destination] = when (existingShipment) {
                null -> combinedShipment {
                    id = "CMB- ${Random.nextInt(1000)}: $destination"
                    status = "Processed!"
                    ordersList.add(currentOrder)
                }
                else -> existingShipment.copy { ordersList.add(currentOrder) }
            }

            if (i % BATCH_SIZE == 0) {
                // Order batch completed. Flush all existing shipments.
                for (value in combinedShipmentMap.values) emit(value)
                combinedShipmentMap.clear()
            }
        }
    }

    private fun initData() {
        orderMap["102"] = order {
            id = "102"
            price = 180_000
            destination = "Mountain View, CA"
            items.addAll(arrayOf("Google Pixel 3A", "Mac Book Pro").asIterable())
        }
        orderMap["103"] = order {
            id = "103"
            price = 40_000
            destination = "San Jose, CA"
            items.add("Apple Watch S4")
        }
        orderMap["104"] = order {
            id = "104"
            price = 40_000
            destination = "Mountain View, CA"
            items.addAll(arrayOf("Google Home Mini", "Google Nest Hub").asIterable())
        }
        orderMap["105"] = order {
            id = "105"
            price = 3_000
            destination = "San Jose, CA"
            items.add("Amazon Echo")
        }
        orderMap["106"] = order {
            id = "106"
            price = 30_000
            destination = "Mountain View, CA"
            items.addAll(arrayOf("Amazon Echo", "Apple iPhone XS").asIterable())
        }
    }
}