//! Transport layer abstraction for MQTT connections.
//!
//! This module provides a unified interface for different transport types:
//! - TCP (plain)
//! - TLS (secure)
//! - WebSocket
//! - WebSocket over TLS

use std::io;
use std::pin::Pin;
use std::task::{Context, Poll};
use tokio::io::{AsyncRead, AsyncWrite, ReadBuf};
use tokio::net::TcpStream;

#[cfg(feature = "tls")]
use tokio_rustls::client::TlsStream;

#[cfg(feature = "websocket")]
use tokio_tungstenite::{MaybeTlsStream, WebSocketStream};

/// Transport type enumeration.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TransportType {
    /// Plain TCP connection.
    Tcp,
    /// TLS encrypted connection.
    #[cfg(feature = "tls")]
    Tls,
    /// WebSocket connection.
    #[cfg(feature = "websocket")]
    WebSocket,
    /// WebSocket over TLS connection.
    #[cfg(all(feature = "websocket", feature = "tls"))]
    WebSocketTls,
}

impl TransportType {
    /// Parse transport type from URL scheme.
    pub fn from_scheme(scheme: &str) -> Option<Self> {
        match scheme.to_lowercase().as_str() {
            "tcp" | "mqtt" | "" => Some(TransportType::Tcp),
            #[cfg(feature = "tls")]
            "tls" | "mqtts" | "ssl" => Some(TransportType::Tls),
            #[cfg(feature = "websocket")]
            "ws" => Some(TransportType::WebSocket),
            #[cfg(all(feature = "websocket", feature = "tls"))]
            "wss" => Some(TransportType::WebSocketTls),
            _ => None,
        }
    }

    /// Get default port for this transport type.
    pub fn default_port(&self) -> u16 {
        match self {
            TransportType::Tcp => 1883,
            #[cfg(feature = "tls")]
            TransportType::Tls => 8883,
            #[cfg(feature = "websocket")]
            TransportType::WebSocket => 80,
            #[cfg(all(feature = "websocket", feature = "tls"))]
            TransportType::WebSocketTls => 443,
        }
    }
}

/// A unified transport that wraps different connection types.
pub enum Transport {
    /// Plain TCP stream.
    Tcp(TcpStream),
    /// TLS stream.
    #[cfg(feature = "tls")]
    Tls(Box<TlsStream<TcpStream>>),
    /// WebSocket stream.
    #[cfg(feature = "websocket")]
    WebSocket(Box<WebSocketStream<MaybeTlsStream<TcpStream>>>),
}

impl AsyncRead for Transport {
    fn poll_read(
        self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &mut ReadBuf<'_>,
    ) -> Poll<io::Result<()>> {
        match self.get_mut() {
            Transport::Tcp(stream) => Pin::new(stream).poll_read(cx, buf),
            #[cfg(feature = "tls")]
            Transport::Tls(stream) => Pin::new(stream.as_mut()).poll_read(cx, buf),
            #[cfg(feature = "websocket")]
            Transport::WebSocket(_) => {
                // WebSocket requires message-based reading
                // This is handled separately via WebSocket frames
                Poll::Ready(Err(io::Error::new(
                    io::ErrorKind::Unsupported,
                    "use websocket_read instead",
                )))
            }
        }
    }
}

impl AsyncWrite for Transport {
    fn poll_write(
        self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &[u8],
    ) -> Poll<io::Result<usize>> {
        match self.get_mut() {
            Transport::Tcp(stream) => Pin::new(stream).poll_write(cx, buf),
            #[cfg(feature = "tls")]
            Transport::Tls(stream) => Pin::new(stream.as_mut()).poll_write(cx, buf),
            #[cfg(feature = "websocket")]
            Transport::WebSocket(_) => {
                // WebSocket requires message-based writing
                Poll::Ready(Err(io::Error::new(
                    io::ErrorKind::Unsupported,
                    "use websocket_write instead",
                )))
            }
        }
    }

    fn poll_flush(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<io::Result<()>> {
        match self.get_mut() {
            Transport::Tcp(stream) => Pin::new(stream).poll_flush(cx),
            #[cfg(feature = "tls")]
            Transport::Tls(stream) => Pin::new(stream.as_mut()).poll_flush(cx),
            #[cfg(feature = "websocket")]
            Transport::WebSocket(_) => Poll::Ready(Ok(())),
        }
    }

    fn poll_shutdown(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<io::Result<()>> {
        match self.get_mut() {
            Transport::Tcp(stream) => Pin::new(stream).poll_shutdown(cx),
            #[cfg(feature = "tls")]
            Transport::Tls(stream) => Pin::new(stream.as_mut()).poll_shutdown(cx),
            #[cfg(feature = "websocket")]
            Transport::WebSocket(_) => Poll::Ready(Ok(())),
        }
    }
}

#[cfg(feature = "tls")]
pub mod tls {
    //! TLS configuration and utilities.

    use std::io;
    use std::sync::Arc;
    use tokio::net::TcpStream;
    use tokio_rustls::rustls::{ClientConfig, RootCertStore};
    use tokio_rustls::TlsConnector;

    /// TLS configuration for client connections.
    #[derive(Clone)]
    pub struct TlsConfig {
        /// The TLS connector.
        pub connector: TlsConnector,
    }

    impl TlsConfig {
        /// Create a new TLS config with default settings.
        pub fn new() -> io::Result<Self> {
            let root_store = RootCertStore {
                roots: webpki_roots::TLS_SERVER_ROOTS.to_vec(),
            };

            let config = ClientConfig::builder()
                .with_root_certificates(root_store)
                .with_no_client_auth();

            Ok(Self {
                connector: TlsConnector::from(Arc::new(config)),
            })
        }

        /// Create a TLS config that skips certificate verification.
        /// **WARNING: This is insecure and should only be used for testing!**
        pub fn insecure() -> Self {
            use tokio_rustls::rustls::client::danger::{
                HandshakeSignatureValid, ServerCertVerified, ServerCertVerifier,
            };
            use tokio_rustls::rustls::pki_types::{CertificateDer, ServerName, UnixTime};
            use tokio_rustls::rustls::{DigitallySignedStruct, SignatureScheme};

            #[derive(Debug)]
            struct InsecureVerifier;

            impl ServerCertVerifier for InsecureVerifier {
                fn verify_server_cert(
                    &self,
                    _end_entity: &CertificateDer<'_>,
                    _intermediates: &[CertificateDer<'_>],
                    _server_name: &ServerName<'_>,
                    _ocsp_response: &[u8],
                    _now: UnixTime,
                ) -> Result<ServerCertVerified, tokio_rustls::rustls::Error> {
                    Ok(ServerCertVerified::assertion())
                }

                fn verify_tls12_signature(
                    &self,
                    _message: &[u8],
                    _cert: &CertificateDer<'_>,
                    _dss: &DigitallySignedStruct,
                ) -> Result<HandshakeSignatureValid, tokio_rustls::rustls::Error> {
                    Ok(HandshakeSignatureValid::assertion())
                }

                fn verify_tls13_signature(
                    &self,
                    _message: &[u8],
                    _cert: &CertificateDer<'_>,
                    _dss: &DigitallySignedStruct,
                ) -> Result<HandshakeSignatureValid, tokio_rustls::rustls::Error> {
                    Ok(HandshakeSignatureValid::assertion())
                }

                fn supported_verify_schemes(&self) -> Vec<SignatureScheme> {
                    vec![
                        SignatureScheme::RSA_PKCS1_SHA256,
                        SignatureScheme::RSA_PKCS1_SHA384,
                        SignatureScheme::RSA_PKCS1_SHA512,
                        SignatureScheme::ECDSA_NISTP256_SHA256,
                        SignatureScheme::ECDSA_NISTP384_SHA384,
                        SignatureScheme::RSA_PSS_SHA256,
                        SignatureScheme::RSA_PSS_SHA384,
                        SignatureScheme::RSA_PSS_SHA512,
                        SignatureScheme::ED25519,
                    ]
                }
            }

            let config = ClientConfig::builder()
                .dangerous()
                .with_custom_certificate_verifier(Arc::new(InsecureVerifier))
                .with_no_client_auth();

            Self {
                connector: TlsConnector::from(Arc::new(config)),
            }
        }

        /// Connect to a TLS server.
        pub async fn connect(
            &self,
            stream: TcpStream,
            domain: &str,
        ) -> io::Result<tokio_rustls::client::TlsStream<TcpStream>> {
            use tokio_rustls::rustls::pki_types::ServerName;

            let domain = ServerName::try_from(domain.to_string())
                .map_err(|_| io::Error::new(io::ErrorKind::InvalidInput, "invalid domain name"))?;

            self.connector.connect(domain, stream).await
        }
    }

    impl Default for TlsConfig {
        fn default() -> Self {
            Self::new().expect("failed to create default TLS config")
        }
    }
}

#[cfg(feature = "websocket")]
pub mod websocket {
    //! WebSocket utilities.

    use futures_util::{SinkExt, StreamExt};
    use std::io;
    use tokio::net::TcpStream;
    use tokio_tungstenite::tungstenite::protocol::Message;
    use tokio_tungstenite::{MaybeTlsStream, WebSocketStream};

    /// Connect to a WebSocket server.
    pub async fn connect(
        url: &str,
    ) -> io::Result<WebSocketStream<MaybeTlsStream<TcpStream>>> {
        let (ws_stream, _) = tokio_tungstenite::connect_async(url)
            .await
            .map_err(|e| io::Error::new(io::ErrorKind::ConnectionRefused, e))?;
        Ok(ws_stream)
    }

    /// Read binary data from a WebSocket stream.
    pub async fn read(
        ws: &mut WebSocketStream<MaybeTlsStream<TcpStream>>,
    ) -> io::Result<Vec<u8>> {
        loop {
            match ws.next().await {
                Some(Ok(Message::Binary(data))) => return Ok(data),
                Some(Ok(Message::Ping(data))) => {
                    // Respond to ping
                    ws.send(Message::Pong(data))
                        .await
                        .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;
                }
                Some(Ok(Message::Close(_))) => {
                    return Err(io::Error::new(
                        io::ErrorKind::ConnectionAborted,
                        "connection closed",
                    ))
                }
                Some(Ok(_)) => continue, // Ignore other message types
                Some(Err(e)) => return Err(io::Error::new(io::ErrorKind::Other, e)),
                None => {
                    return Err(io::Error::new(
                        io::ErrorKind::UnexpectedEof,
                        "stream ended",
                    ))
                }
            }
        }
    }

    /// Write binary data to a WebSocket stream.
    pub async fn write(
        ws: &mut WebSocketStream<MaybeTlsStream<TcpStream>>,
        data: &[u8],
    ) -> io::Result<()> {
        ws.send(Message::Binary(data.to_vec()))
            .await
            .map_err(|e| io::Error::new(io::ErrorKind::Other, e))
    }
}
