import 'package:flutter/material.dart';
import '../../../../core/theme/app_theme.dart';

class RecentTripsWidget extends StatelessWidget {
  const RecentTripsWidget({super.key});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Recent Trips',
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
          ),
          const SizedBox(height: 12),
          _TripCard(
            from: 'Rajiv Chowk Metro',
            to: 'Saket Mall',
            mode: 'Metro',
            fare: '₹30',
            date: 'Today, 10:30 AM',
            icon: Icons.subway,
            color: const Color(0xFF8B5CF6),
          ),
          const SizedBox(height: 12),
          _TripCard(
            from: 'Home',
            to: 'Cyber Hub',
            mode: 'Auto',
            fare: '₹85',
            date: 'Yesterday, 9:15 AM',
            icon: Icons.local_taxi,
            color: const Color(0xFFF59E0B),
          ),
          const SizedBox(height: 12),
          _TripCard(
            from: 'IGI Airport T3',
            to: 'Connaught Place',
            mode: 'Cab (Uber Go)',
            fare: '₹320',
            date: 'Jul 10, 6:45 PM',
            icon: Icons.car_rental,
            color: AppTheme.accentGreen,
          ),
        ],
      ),
    );
  }
}

class _TripCard extends StatelessWidget {
  final String from;
  final String to;
  final String mode;
  final String fare;
  final String date;
  final IconData icon;
  final Color color;

  const _TripCard({
    required this.from,
    required this.to,
    required this.mode,
    required this.fare,
    required this.date,
    required this.icon,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(16),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.05),
            blurRadius: 10,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(12),
            ),
            child: Icon(
              icon,
              color: color,
            ),
          ),
          const SizedBox(width: 16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(
                      mode,
                      style: Theme.of(context).textTheme.labelMedium?.copyWith(
                            color: color,
                            fontWeight: FontWeight.w600,
                          ),
                    ),
                    const Spacer(),
                    Text(
                      fare,
                      style: Theme.of(context).textTheme.titleMedium?.copyWith(
                            fontWeight: FontWeight.w700,
                            color: AppTheme.textPrimary,
                          ),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  '$from → $to',
                  style: Theme.of(context).textTheme.bodyMedium,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(
                  date,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: AppTheme.textSecondary,
                      ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          const Icon(
            Icons.chevron_right,
            color: AppTheme.textSecondary,
          ),
        ],
      ),
    );
  }
}
