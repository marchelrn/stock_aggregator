import { useEffect, useState } from 'react'
import type { Broker } from '../types'

interface BrokerFormProps {
  brokers?: Broker[]
  onSubmit?: (payload: { name: string; cash: number }) => void | Promise<void>
  onCashSubmit?: (payload: { brokerId: number | null; cashAmount: number }) => void | Promise<void>
}

export default function BrokerForm({ brokers = [], onSubmit, onCashSubmit }: BrokerFormProps) {
  const [name, setName] = useState('')
  const [cash, setCash] = useState(0)
  const [brokerId, setBrokerId] = useState<number | null>(null)
  const [cashAmount, setCashAmount] = useState(0)
  const [showForm, setShowForm] = useState(false)
  const [showCashForm, setShowCashForm] = useState(false)

  const brokerOptions = brokers || []

  const toggleForm = (formType: 'Broker' | 'Cash') => {
    if (formType === 'Broker') {
      setShowForm((v) => !v)
      setShowCashForm(false)
    } else {
      setShowCashForm((v) => !v)
      setShowForm(false)
    }
  }

  // When the cash form opens, preselect the first broker.
  useEffect(() => {
    if (!showCashForm) return
    setBrokerId(brokerOptions[0] ? Number(brokerOptions[0].id) : null)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [showCashForm])

  // Keep the selected broker valid as the list changes.
  useEffect(() => {
    if (!brokerOptions.some((b) => Number(b.id) === Number(brokerId))) {
      setBrokerId(brokerOptions[0] ? Number(brokerOptions[0].id) : null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [brokers])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await onSubmit?.({ name: name.trim(), cash })
      setName('')
      setCash(0)
      setShowForm(false)
    } catch (error) {
      console.error('Error submitting broker form:', error)
    }
  }

  const handleCashSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await onCashSubmit?.({ brokerId, cashAmount })
      setBrokerId(null)
      setCashAmount(0)
    } catch (error) {
      console.error('Error submitting cash form:', error)
    }
  }

  return (
    <div>
      <button
        className="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800"
        onClick={() => toggleForm('Broker')}
      >
        {showForm ? 'Close Broker Form' : 'Add New Broker'}
      </button>
      <button
        className="ml-3 rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800"
        onClick={() => toggleForm('Cash')}
      >
        {showCashForm ? 'Close Cash Form' : 'Add Broker Cash'}
      </button>

      {showForm && (
        <form
          className="mt-3 grid content-start gap-2 rounded-xl border border-slate-300 bg-white p-3"
          id="addBroker"
          onSubmit={handleSubmit}
        >
          <h3 className="text-base font-semibold">Add Broker</h3>
          <label className="text-sm text-slate-600" htmlFor="broker-name">
            Broker Name
          </label>
          <input
            id="broker-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            type="text"
            required
            className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
          />
          <label className="text-sm text-slate-600" htmlFor="broker-cash">
            Cash
          </label>
          <input
            id="broker-cash"
            value={cash}
            onChange={(e) => setCash(Number(e.target.value))}
            type="number"
            min="0"
            step="0.01"
            className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
          />
          <button
            type="submit"
            className="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800"
          >
            Add Broker
          </button>
        </form>
      )}

      {showCashForm && (
        <form
          className="mt-3 grid content-start gap-2 rounded-xl border border-slate-300 bg-white p-3"
          id="addCash"
          onSubmit={handleCashSubmit}
        >
          <h3 className="text-base font-semibold">Add Broker Cash</h3>
          <label className="text-sm text-slate-600" htmlFor="tx-broker">
            Broker Name
          </label>
          <select
            id="tx-broker"
            value={brokerId ?? ''}
            onChange={(e) => setBrokerId(e.target.value === '' ? null : Number(e.target.value))}
            required
            className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
          >
            {brokerOptions.length === 0 ? (
              <option value="">Tidak ada broker tersedia</option>
            ) : (
              brokerOptions.map((broker) => (
                <option key={broker.id} value={broker.id}>
                  {broker.name} (ID: {broker.id})
                </option>
              ))
            )}
          </select>
          <label className="text-sm text-slate-600" htmlFor="cash-amount">
            Cash Amount
          </label>
          <input
            id="cash-amount"
            value={cashAmount}
            onChange={(e) => setCashAmount(Number(e.target.value))}
            type="number"
            min="0"
            step="0.01"
            className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
          />
          <button
            type="submit"
            className="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800"
          >
            Add Cash
          </button>
        </form>
      )}
    </div>
  )
}
