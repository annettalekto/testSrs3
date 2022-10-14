#ifndef INC_CRC_H_
#define INC_CRC_H_

#include <stdint.h>

uint32_t BU4_CRC_Calculate(const void * data, size_t length);
uint32_t pinCodeFromDate(uint8_t day, uint8_t month, uint8_t year);

#endif /* INC_CRC_H_ */
