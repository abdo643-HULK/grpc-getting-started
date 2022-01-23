package ecommerce

import io.grpc.CallCredentials
import io.grpc.Metadata
import io.grpc.Status
import java.util.concurrent.Executor

class TokenCallCredentials(private val token: String) : CallCredentials() {
    override fun applyRequestMetadata(requestInfo: RequestInfo, appExecutor: Executor, applier: MetadataApplier) {
        appExecutor.execute {
            try {
                val headers = Metadata()
                val authKey = Metadata.Key.of("Authorization", Metadata.ASCII_STRING_MARSHALLER)
                headers.put(authKey, "Bearer $token")
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