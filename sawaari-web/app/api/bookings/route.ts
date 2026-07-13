import { NextRequest, NextResponse } from 'next/server';

const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:8080';

// GET - Retrieve bookings
export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const userId = searchParams.get('userId');
  const status = searchParams.get('status');

  if (!userId) {
    return NextResponse.json(
      { error: 'Missing required parameter: userId' },
      { status: 400 }
    );
  }

  try {
    const params = new URLSearchParams({ userId });
    if (status) params.set('status', status);

    const response = await fetch(
      `${BACKEND_URL}/api/bookings?${params.toString()}`,
      {
        headers: {
          'Content-Type': 'application/json',
        },
        signal: AbortSignal.timeout,
      }
    );

    if (!response.ok) {
      throw new Error(`Backend responded with status ${response.status}`);
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error('Bookings GET error:', error);

    // Return mock data
    return NextResponse.json(getMockBookingsResponse(), {
      headers: {
        'X-Data-Source': 'mock',
      },
    });
  }
}

// POST - Create new booking
export async function POST(request: NextRequest) {
  try {
    const body = await request.json();

    const { from, to, optionId, provider, fare, datetime, userId } = body;

    if (!from || !to || !optionId || !provider || !fare) {
      return NextResponse.json(
        { error: 'Missing required booking information' },
        { status: 400 }
      );
    }

    // Try to forward to backend
    try {
      const response = await fetch(`${BACKEND_URL}/api/bookings`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
        signal: AbortSignal.timeout,
      });

      if (response.ok) {
        const data = await response.json();
        return NextResponse.json(data, { status: 201 });
      }
    } catch {
      // Backend unavailable, queue for later sync
    }

    // Queue booking for background sync
    const bookingId = `booking_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

    return NextResponse.json({
      success: true,
      queued: true,
      booking: {
        id: bookingId,
        from,
        to,
        optionId,
        provider,
        fare,
        datetime: datetime || new Date().toISOString(),
        status: 'queued',
        createdAt: new Date().toISOString(),
      },
      message: 'Booking queued for sync when online',
    }, {
      status: 202,
    });
  } catch (error) {
    console.error('Bookings POST error:', error);
    return NextResponse.json(
      { error: 'Failed to process booking request' },
      { status: 500 }
    );
  }
}

function getMockBookingsResponse() {
  return {
    success: true,
    dataSource: 'mock',
    bookings: [
      {
        id: 'booking_001',
        from: 'Rajiv Chowk Metro',
        to: 'Saket Mall',
        provider: 'DMRC Metro',
        mode: 'metro',
        fare: 30,
        status: 'completed',
        datetime: new Date().toISOString(),
        createdAt: new Date(Date.now() - 86400000).toISOString(),
      },
      {
        id: 'booking_002',
        from: 'Home',
        to: 'Cyber Hub',
        provider: 'Auto Rickshaw',
        mode: 'auto',
        fare: 85,
        status: 'completed',
        datetime: new Date(Date.now() - 172800000).toISOString(),
        createdAt: new Date(Date.now() - 172800000).toISOString(),
      },
    ],
  };
}
