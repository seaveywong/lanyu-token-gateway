import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createPaymentOrder } from '@/api/billing';
import type { PaymentOrder } from '@/api/billing';
import styles from './RechargeDialog.module.css';

const PRESET_AMOUNTS = [10, 50, 100, 500];

interface RechargeDialogProps {
  open: boolean;
  onClose: () => void;
}

function RechargeDialog({ open, onClose }: RechargeDialogProps) {
  const queryClient = useQueryClient();
  const [selectedAmount, setSelectedAmount] = useState<number>(50);
  const [customAmount, setCustomAmount] = useState('');
  const [useCustom, setUseCustom] = useState(false);
  const [paymentMethod, setPaymentMethod] = useState<'alipay' | 'wechat'>('alipay');
  const [step, setStep] = useState<'select' | 'pay'>('select');
  const [order, setOrder] = useState<PaymentOrder | null>(null);
  const [error, setError] = useState<string | null>(null);

  const createMutation = useMutation({
    mutationFn: createPaymentOrder,
    onSuccess: (data) => {
      setOrder(data);
      setStep('pay');
      queryClient.invalidateQueries({ queryKey: ['paymentOrders'] });
    },
    onError: (err: Error) => {
      setError(err.message || '创建订单失败');
    },
  });

  if (!open) return null;

  const amountYuan = useCustom
    ? parseFloat(customAmount) || 0
    : selectedAmount;

  const handleSubmit = () => {
    setError(null);

    if (amountYuan <= 0) {
      setError('请选择或输入有效充值金额');
      return;
    }

    createMutation.mutate({
      payment_method: paymentMethod,
      amount_yuan: Math.round(amountYuan * 100), // Convert yuan to fen
    });
  };

  const handleClose = () => {
    setStep('select');
    setOrder(null);
    setError(null);
    onClose();
  };

  return (
    <div className={styles.overlay} onClick={handleClose}>
      <div className={styles.dialog} onClick={(e) => e.stopPropagation()}>
        <div className={styles.header}>
          <h2 className={styles.title}>
            {step === 'select' ? '账户充值' : '扫码支付'}
          </h2>
          <button className={styles.closeBtn} onClick={handleClose} aria-label="关闭">
            &times;
          </button>
        </div>

        {step === 'select' && (
          <div className={styles.body}>
            {/* Preset amounts */}
            <div className={styles.section}>
              <label className={styles.sectionLabel}>选择充值金额</label>
              <div className={styles.amountGrid}>
                {PRESET_AMOUNTS.map((amt) => (
                  <button
                    key={amt}
                    className={
                      !useCustom && selectedAmount === amt
                        ? styles.amountBtnActive
                        : styles.amountBtn
                    }
                    onClick={() => { setSelectedAmount(amt); setUseCustom(false); }}
                  >
                    &yen;{amt}
                  </button>
                ))}
                <button
                  className={useCustom ? styles.amountBtnActive : styles.amountBtn}
                  onClick={() => setUseCustom(true)}
                >
                  自定义
                </button>
              </div>
              {useCustom && (
                <div className={styles.customAmountWrap}>
                  <span className={styles.yuanPrefix}>&yen;</span>
                  <input
                    className={styles.customInput}
                    type="number"
                    min="1"
                    step="0.01"
                    placeholder="输入金额"
                    value={customAmount}
                    onChange={(e) => setCustomAmount(e.target.value)}
                    autoFocus
                  />
                  <span className={styles.yuanSuffix}>元</span>
                </div>
              )}
            </div>

            {/* Payment method */}
            <div className={styles.section}>
              <label className={styles.sectionLabel}>支付方式</label>
              <div className={styles.paymentMethods}>
                <button
                  className={
                    paymentMethod === 'alipay'
                      ? styles.paymentBtnActive
                      : styles.paymentBtn
                  }
                  onClick={() => setPaymentMethod('alipay')}
                >
                  <span className={styles.paymentIcon}>💙</span>
                  支付宝
                </button>
                <button
                  className={
                    paymentMethod === 'wechat'
                      ? styles.paymentBtnActive
                      : styles.paymentBtn
                  }
                  onClick={() => setPaymentMethod('wechat')}
                >
                  <span className={styles.paymentIcon}>💚</span>
                  微信支付
                </button>
              </div>
            </div>

            {error && <div className={styles.error}>{error}</div>}

            <div className={styles.summary}>
              <span>充值金额：</span>
              <strong>&yen;{amountYuan.toFixed(2)}</strong>
              <span style={{ marginLeft: 8, color: '#6b7280', fontSize: 13 }}>
                ({paymentMethod === 'alipay' ? '支付宝' : '微信支付'})
              </span>
            </div>

            <button
              className={styles.submitBtn}
              onClick={handleSubmit}
              disabled={createMutation.isPending}
            >
              {createMutation.isPending ? '创建订单中...' : '确认充值'}
            </button>
          </div>
        )}

        {step === 'pay' && order && (
          <div className={styles.body}>
            <div className={styles.payInfo}>
              <div className={styles.payAmount}>
                <span className={styles.payAmountLabel}>支付金额</span>
                <span className={styles.payAmountValue}>
                  &yen;{(order.amount_yuan / 100).toFixed(2)}
                </span>
              </div>
              <div className={styles.payOrderNo}>
                订单号：{order.order_no}
              </div>
            </div>

            {/* QR code placeholder */}
            <div className={styles.qrWrapper}>
              {order.qr_code_url ? (
                <img
                  className={styles.qrImage}
                  src={order.qr_code_url}
                  alt="支付二维码"
                />
              ) : (
                <div className={styles.qrPlaceholder}>
                  <div className={styles.qrIcon}>
                    {paymentMethod === 'alipay' ? '💙' : '💚'}
                  </div>
                  <div className={styles.qrText}>
                    {paymentMethod === 'alipay' ? '支付宝' : '微信支付'}扫码支付
                  </div>
                  <div className={styles.qrDesc}>
                    二维码由后端动态生成<br />
                    生产环境将展示真实支付二维码
                  </div>
                  <div className={styles.qrBox}>
                    <div className={styles.qrPattern}>
                      <div className={styles.qrDot} />
                      <div className={styles.qrDot} />
                      <div className={styles.qrDot} />
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className={styles.payHint}>
              {paymentMethod === 'alipay'
                ? '请使用支付宝扫描二维码完成支付'
                : '请使用微信扫描二维码完成支付'}
            </div>

            <button className={styles.backBtn} onClick={handleClose}>
              关闭
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

export default RechargeDialog;
