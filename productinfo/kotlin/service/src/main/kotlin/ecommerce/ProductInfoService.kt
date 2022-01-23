package ecommerce

import com.google.protobuf.StringValue
import com.google.protobuf.stringValue
import io.grpc.Status
import io.grpc.StatusException
import java.util.*

class ProductInfoService : ProductInfoGrpcKt.ProductInfoCoroutineImplBase() {
    private val productMap by lazy { mutableMapOf<String, Product>() }

    override suspend fun addProduct(request: Product): StringValue {
        val uuid = UUID.randomUUID().toString()
        productMap[uuid] = request

        return stringValue { value = uuid }
    }

    override suspend fun getProduct(request: StringValue): Product =
        productMap[request.value] ?: throw StatusException(Status.NOT_FOUND)
}