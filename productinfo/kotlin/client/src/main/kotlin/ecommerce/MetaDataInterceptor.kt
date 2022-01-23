package ecommerce

import io.grpc.*
import io.grpc.ForwardingClientCall.SimpleForwardingClientCall
import io.grpc.Metadata.ASCII_STRING_MARSHALLER


class MetaDataInterceptor : ClientInterceptor {
    override fun <ReqT, RespT> interceptCall(
        method: MethodDescriptor<ReqT, RespT>,
        callOptions: CallOptions,
        next: Channel
    ): ClientCall<ReqT, RespT> {
        return object : SimpleForwardingClientCall<ReqT, RespT>(next.newCall(method, callOptions)) {
            override fun start(responseListener: Listener<RespT>, headers: Metadata) {
                headers.put(Metadata.Key.of("MY_MD_1", ASCII_STRING_MARSHALLER), "This is metadata of MY_MD_1")
                super.start(responseListener, headers)
            }
        }
    }
}