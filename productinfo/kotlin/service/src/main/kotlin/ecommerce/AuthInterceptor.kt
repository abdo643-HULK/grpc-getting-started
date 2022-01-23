package ecommerce

import io.grpc.*
import io.grpc.Metadata.ASCII_STRING_MARSHALLER
import java.util.*
import java.util.logging.Logger


class AuthInterceptor : ServerInterceptor {
    companion object {
        private val ADMIN_USER_CREDENTIALS = "admin:admin"
        private val USER_ID_CTX_KEY = Context.key<String>("userId")
        private val ADMIN_USER_ID = "admin"
        private val logger = Logger.getLogger(OrderServerInterceptor::class.java.name)
        private val ADMIN_USER_TOKEN = "some-secret-token"
        private val authType = "oauth"
    }

    override fun <ReqT, RespT> interceptCall(
        call: ServerCall<ReqT, RespT>,
        headers: Metadata,
        next: ServerCallHandler<ReqT, RespT>
    ): ServerCall.Listener<ReqT> {
        val basicAuthString = headers.get(Metadata.Key.of("authorization", ASCII_STRING_MARSHALLER))
            ?: run {
                call.close(
                    Status.UNAUTHENTICATED.withDescription("Basic authentication value is missing in Metadata"),
                    headers
                )

                return object : ServerCall.Listener<ReqT>() {}
            }

        return if (validUser(basicAuthString)) {
            val ctx = Context.current().withValue(USER_ID_CTX_KEY, ADMIN_USER_ID)
            Contexts.interceptCall(ctx, call, headers, next)
        } else {
            logger.info("Verification failed - Unauthenticated!")
            call.close(Status.UNAUTHENTICATED.withDescription("Invalid basic credentials"), headers)
            object : ServerCall.Listener<ReqT>() {}
        }
    }

    private fun validUser(authString: String): Boolean {
        if (authType == "basic") {
            val token = authString.substring("Basic ".length).trim()
            val byteArray = Base64.getDecoder().decode(token.toByteArray())
            return ADMIN_USER_CREDENTIALS == byteArray.toString()
        }

        val token = authString.substring("Bearer ".length).trim()
        return ADMIN_USER_TOKEN == token
    }
}