import 'dart:io';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import '../../models/models.dart';

/// API client for Sawaari backend using Dio with retry logic
class ApiClient {
  late final Dio _dio;

  // Base URL - should be configured per environment
  static const String baseUrl = 'https://api.sawaari.in/v1';

  ApiClient({String? baseUrlOverride}) {
    _dio = Dio(BaseOptions(
      baseUrl: baseUrlOverride ?? baseUrl,
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 30),
      sendTimeout: const Duration(seconds: 30),
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
    ));

    _setupInterceptors();
  }

  void _setupInterceptors() {
    _dio.interceptors.addAll([
      _RetryInterceptor(dio: _dio),
      LogInterceptor(
        requestBody: true,
        responseBody: true,
        error: true,
        logPrint: (o) => debugPrint('ApiClient: $o'),
      ),
    ]);
  }

  void setAuthToken(String token) {
    _dio.options.headers['Authorization'] = 'Bearer $token';
  }

  void clearAuthToken() {
    _dio.options.headers.remove('Authorization');
  }

  // ============ Compare / Trip APIs ============

  /// Compare prices across all providers for a trip
  Future<List<RideOption>> comparePrices({
    required double fromLat,
    required double fromLng,
    required double toLat,
    required double toLng,
    TripPreferences? preferences,
  }) async {
    try {
      final response = await _dio.post(
        '/compare',
        data: {
          'from': {'lat': fromLat, 'lng': fromLng},
          'to': {'lat': toLat, 'lng': toLng},
          'preferences': preferences?.toJson(),
        },
      );

      final List<dynamic> data = response.data['options'];
      return data.map((json) => RideOption.fromJson(json)).toList();
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  // ============ Stops APIs ============

  /// Get nearby transit stops
  Future<List<TransitStop>> getNearbyStops({
    required double lat,
    required double lng,
    int radius = 500,
  }) async {
    try {
      final response = await _dio.get(
        '/stops/nearby',
        queryParameters: {
          'lat': lat,
          'lng': lng,
          'r': radius,
        },
      );

      final List<dynamic> data = response.data['stops'];
      return data.map((json) => TransitStop.fromJson(json)).toList();
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Get route details
  Future<Map<String, dynamic>> getRouteDetails(String routeId) async {
    try {
      final response = await _dio.get('/routes/$routeId');
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  // ============ Booking APIs ============

  /// Create a booking
  Future<Booking> createBooking({
    required String optionId,
    required String bookingRail,
    required String pickupAddress,
    required String dropoffAddress,
    required double pickupLat,
    required double pickupLng,
    required double dropoffLat,
    required double dropoffLng,
    required PaymentMethod paymentMethod,
    DateTime? scheduledAt,
  }) async {
    try {
      final response = await _dio.post(
        '/bookings',
        data: {
          'option_id': optionId,
          'rail': bookingRail,
          'pickup': {
            'address': pickupAddress,
            'lat': pickupLat,
            'lng': pickupLng,
          },
          'dropoff': {
            'address': dropoffAddress,
            'lat': dropoffLat,
            'lng': dropoffLng,
          },
          'payment_method': paymentMethod.name,
          'scheduled_at': scheduledAt?.toIso8601String(),
        },
      );

      return Booking.fromJson(response.data);
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Get booking status
  Future<Booking> getBooking(String bookingId) async {
    try {
      final response = await _dio.get('/bookings/$bookingId');
      return Booking.fromJson(response.data);
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Cancel a booking
  Future<void> cancelBooking(String bookingId) async {
    try {
      await _dio.post('/bookings/$bookingId/cancel');
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  // ============ Wallet APIs ============

  /// Get user's tickets
  Future<List<Map<String, dynamic>>> getTickets() async {
    try {
      final response = await _dio.get('/wallet/tickets');
      return List<Map<String, dynamic>>>.from(response.data['tickets']);
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Download ticket as PDF
  Future<String> downloadTicketPdf(String ticketId) async {
    try {
      final response = await _dio.get('/wallet/tickets/$ticketId.pdf');
      return response.data['url'] as String;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  // ============ User APIs ============

  /// Get user profile
  Future<Map<String, dynamic>> getUserProfile() async {
    try {
      final response = await _dio.get('/users/me');
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Update user preferences
  Future<void> updatePreferences(TripPreferences preferences) async {
    try {
      await _dio.patch('/users/me/preferences', data: preferences.toJson());
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  // ============ Feedback APIs ============

  /// Submit quote feedback (actual fare vs quoted fare)
  Future<void> submitQuoteFeedback({
    required String quoteId,
    required double actualFare,
  }) async {
    try {
      await _dio.post(
        '/feedback/quote',
        data: {
          'quote_id': quoteId,
          'actual_fare': actualFare,
        },
      );
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  // ============ Error Handling ============

  ApiException _handleError(DioException e) {
    switch (e.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
        return ApiException(
          'Connection timed out. Please check your internet.',
          statusCode: null,
        );
      case DioExceptionType.connectionError:
        return ApiException(
          'No internet connection.',
          statusCode: null,
        );
      case DioExceptionType.badResponse:
        final statusCode = e.response?.statusCode;
        final message = _extractErrorMessage(e.response);
        return ApiException(message, statusCode: statusCode);
      case DioExceptionType.cancel:
        return ApiException('Request cancelled.');
      default:
        return ApiException('Something went wrong. Please try again.');
    }
  }

  String _extractErrorMessage(Response? response) {
    if (response == null) return 'Unknown error';

    final data = response.data;
    if (data is Map && data.containsKey('message')) {
      return data['message'] as String;
    }
    if (data is Map && data.containsKey('error')) {
      return data['error'] as String;
    }

    switch (response.statusCode) {
      case 400:
        return 'Invalid request.';
      case 401:
        return 'Please login again.';
      case 403:
        return 'Access denied.';
      case 404:
        return 'Resource not found.';
      case 429:
        return 'Too many requests. Please wait.';
      case 500:
        return 'Server error. Please try again later.';
      default:
        return 'Something went wrong.';
    }
  }
}

/// Custom retry interceptor with exponential backoff
class _RetryInterceptor extends Interceptor {
  final Dio dio;
  final int maxRetries;
  final int retryDelayMs;

  _RetryInterceptor({
    required this.dio,
    this.maxRetries = 3,
    this.retryDelayMs = 1000,
  });

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final extra = err.requestOptions.extra;
    final retryCount = extra['retryCount'] as int? ?? 0;

    // Don't retry on auth errors or client errors (except 429)
    if (_shouldNotRetry(err)) {
      return handler.next(err);
    }

    // Check if we have retries left
    if (retryCount >= maxRetries) {
      return handler.next(err);
    }

    // Check network connectivity
    if (!await _hasNetwork()) {
      return handler.next(err);
    }

    // Exponential backoff
    final delayMs = retryDelayMs * (1 << retryCount);
    await Future.delayed(Duration(milliseconds: delayMs));

    // Update retry count and retry
    err.requestOptions.extra['retryCount'] = retryCount + 1;

    try {
      final response = await dio.fetch(err.requestOptions);
      return handler.resolve(response);
    } catch (e) {
      return handler.next(err);
    }
  }

  bool _shouldNotRetry(DioException err) {
    final statusCode = err.response?.statusCode;
    if (statusCode == null) {
      // Network error - should retry
      return false;
    }
    // Don't retry on auth errors
    if (statusCode == 401 || statusCode == 403) {
      return true;
    }
    // Don't retry on client errors (except 429 rate limiting)
    if (statusCode >= 400 && statusCode < 500 && statusCode != 429) {
      return true;
    }
    return false;
  }

  Future<bool> _hasNetwork() async {
    try {
      final result = await InternetAddress.lookup('api.sawaari.in');
      return result.isNotEmpty && result[0].rawAddress.isNotEmpty;
    } catch (_) {
      return false;
    }
  }
}

/// Custom API exception
class ApiException implements Exception {
  final String message;
  final int? statusCode;

  ApiException(this.message, {this.statusCode});

  @override
  String toString() => message;
}
