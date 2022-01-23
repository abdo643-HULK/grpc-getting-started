package ecommerce

import io.grpc.CallCredentials
import io.grpc.Metadata
import io.grpc.Status
import java.util.*
import java.util.concurrent.Executor

class BasicCallCredentials(
    username: String,
    password: String
) : CallCredentials() {
    private val credentials = "$username:$password"

    override fun applyRequestMetadata(requestInfo: RequestInfo, appExecutor: Executor, applier: MetadataApplier) {
        appExecutor.execute {
            try {
                val headers = Metadata()
                val authKey = Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER)
                headers.put(authKey, "Basic ${Base64.getEncoder().encodeToString(credentials.toByteArray())}")
                applier.apply(headers)
            } catch (_e: Throwable) {
                applier.fail(Status.UNAUTHENTICATED.withCause(_e))
            }
        }
    }

    override fun thisUsesUnstableApi() {
        TODO("Not yet implemented")
    }

}