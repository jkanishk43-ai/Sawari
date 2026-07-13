import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/models.dart';
import 'trip_provider.dart';

/// Booking state
class BookingState {
  final RideOption? selectedOption;
  final PaymentMethod paymentMethod;
  final bool isLoading;
  final Booking? currentBooking;
  final String? error;
  final bool saheliDiscountApplied;

  const BookingState({
    this.selectedOption,
    this.paymentMethod = PaymentMethod.upi,
    this.isLoading = false,
    this.currentBooking,
    this.error,
    this.saheliDiscountApplied = false,
  });

  BookingState copyWith({
    RideOption? selectedOption,
    PaymentMethod? paymentMethod,
    bool? isLoading,
    Booking? currentBooking,
    String? error,
    bool? saheliDiscountApplied,
  }) {
    return BookingState(
      selectedOption: selectedOption ?? this.selectedOption,
      paymentMethod: paymentMethod ?? this.paymentMethod,
      isLoading: isLoading ?? this.isLoading,
      currentBooking: currentBooking ?? this.currentBooking,
      error: error,
      saheliDiscountApplied: saheliDiscountApplied ?? this.saheliDiscountApplied,
    );
  }

  double get finalFare {
    if (selectedOption == null) return 0;

    double fare = selectedOption!.fareMin;
    if (saheliDiscountApplied && selectedOption!.fareBreakdown != null) {
      fare -= selectedOption!.fareBreakdown!.saheliDiscount;
    }
    return fare.clamp(0, double.infinity);
  }
}

/// Booking notifier
class BookingNotifier extends StateNotifier<BookingState> {
  final Ref _ref;

  BookingNotifier(this._ref) : super(const BookingState());

  void selectOption(RideOption option) {
    state = state.copyWith(selectedOption: option);
  }

  void setPaymentMethod(PaymentMethod method) {
    state = state.copyWith(paymentMethod: method);
  }

  void toggleSaheliDiscount() {
    state = state.copyWith(saheliDiscountApplied: !state.saheliDiscountApplied);
  }

  Future<bool> confirmBooking({
    required String pickupAddress,
    required String dropoffAddress,
    required double pickupLat,
    required double pickupLng,
    required double dropoffLat,
    required double dropoffLng,
  }) async {
    if (state.selectedOption == null) {
      state = state.copyWith(error: 'No ride option selected');
      return false;
    }

    state = state.copyWith(isLoading: true, error: null);

    try {
      final apiClient = _ref.read(apiClientProvider);

      final booking = await apiClient.createBooking(
        optionId: state.selectedOption!.id,
        bookingRail: state.selectedOption!.bookingRail ?? 'deeplink',
        pickupAddress: pickupAddress,
        dropoffAddress: dropoffAddress,
        pickupLat: pickupLat,
        pickupLng: pickupLng,
        dropoffLat: dropoffLat,
        dropoffLng: dropoffLng,
        paymentMethod: state.paymentMethod,
      );

      state = state.copyWith(
        isLoading: false,
        currentBooking: booking,
      );
      return true;
    } catch (e) {
      // For demo, create a mock booking
      final mockBooking = Booking(
        id: 'SAW-${DateTime.now().millisecondsSinceEpoch}',
        tripId: 'trip_${DateTime.now().millisecondsSinceEpoch}',
        rideOption: state.selectedOption!,
        pickupAddress: pickupAddress,
        dropoffAddress: dropoffAddress,
        pickupLatitude: pickupLat,
        pickupLongitude: pickupLng,
        dropoffLatitude: dropoffLat,
        dropoffLongitude: dropoffLng,
        createdAt: DateTime.now(),
        status: BookingStatus.confirmed,
        paymentMethod: state.paymentMethod,
        finalFare: state.finalFare,
        ticketId: 'TKT-${DateTime.now().millisecondsSinceEpoch.toString().substring(5)}',
      );

      state = state.copyWith(
        isLoading: false,
        currentBooking: mockBooking,
      );
      return true;
    }
  }

  void clearBooking() {
    state = const BookingState();
  }
}

/// Booking provider
final bookingProvider = StateNotifierProvider<BookingNotifier, BookingState>((ref) {
  return BookingNotifier(ref);
});

/// Provider for booking history
final bookingHistoryProvider = FutureProvider<List<Booking>>((ref) async {
  // TODO: Replace with actual API call
  return [];
});
