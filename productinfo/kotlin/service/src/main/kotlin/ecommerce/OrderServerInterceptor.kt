package ecommerce

import io.grpc.*
import io.grpc.ForwardingServerCall.SimpleForwardingServerCall
import java.util.logging.Logger


class OrderServerInterceptor : ServerInterceptor {
    private val logger = Logger.getLogger(OrderServerInterceptor::class.java.name)

    override fun <ReqT, RespT> interceptCall(
        call: ServerCall<ReqT, RespT>,
        headers: Metadata,
        next: ServerCallHandler<ReqT, RespT>
    ): ServerCall.Listener<ReqT> {
        logger.info("======= [Server Interceptor] : Remote Method Invoked - ${call.methodDescriptor.fullMethodName}\n")

        val serverCall = OrderMgtServerCall(call)

        return OrderServerCallListener(next.startCall(serverCall, headers))
    }
}

class OrderServerCallListener<R> internal constructor(private val delegate: ServerCall.Listener<R>) :
    ForwardingServerCallListener<R>() {
    private val logger = Logger.getLogger(OrderServerCallListener::class.java.name)

    override fun delegate(): ServerCall.Listener<R> {
        return delegate
    }

    override fun onMessage(message: R) {
        super.onMessage(message)
        logger.info("Message Received from Client -> Service $message")
    }
}

class OrderMgtServerCall<ReqT, RespT> internal constructor(delegate: ServerCall<ReqT, RespT>?) :
    SimpleForwardingServerCall<ReqT, RespT>(delegate) {
    private val logger = Logger.getLogger(OrderMgtServerCall::class.java.name)

    override fun sendMessage(message: RespT) {
        super.sendMessage(message)
        logger.info("Message Send from Service -> Client : $message")
    }
}